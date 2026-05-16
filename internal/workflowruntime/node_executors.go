package workflowruntime

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"flow-anything/internal/flowengine"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type ConnectorOperationNodeExecutor struct {
	invoker ConnectorInvoker
}

func NewConnectorOperationNodeExecutor(invoker ConnectorInvoker) *ConnectorOperationNodeExecutor {
	return &ConnectorOperationNodeExecutor{invoker: invoker}
}

func (e *ConnectorOperationNodeExecutor) ExecuteNode(ctx context.Context, request flowengine.NodeExecutionRequest) (flowengine.NodeResult, error) {
	if e.invoker == nil {
		return flowengine.NodeResult{}, apperrors.New(apperrors.CodeUnavailable, "connector invoker is not configured")
	}
	operationID := id.ID(stringConfig(request.Node.Config, "operation_id"))
	if operationID.Empty() {
		operationID = id.ID(stringConfig(request.Node.Config, "connector_operation_id"))
	}
	if operationID.Empty() {
		return flowengine.NodeResult{}, apperrors.New(apperrors.CodeInvalidArgument, "connector operation node requires operation_id")
	}

	result, err := e.invoker.Invoke(ctx, connector.InvokeRequest{
		ID:          id.New("wfcinvoke"),
		TenantID:    request.Run.TenantID,
		OperationID: operationID,
		Args:        request.Input,
		TraceID:     request.Run.TraceID,
	})
	if err != nil {
		return flowengine.NodeResult{}, err
	}

	output := map[string]any{
		"request_id":  result.RequestID.String(),
		"success":     result.Success,
		"data":        result.Data,
		"error_code":  result.ErrorCode,
		"finished_at": result.FinishedAt,
	}
	if !result.Success {
		return flowengine.NodeResult{Output: output}, nil
	}

	return flowengine.NodeResult{Output: output}, nil
}

type ToolNodeExecutor struct {
	runtime ToolRuntime
}

func NewToolNodeExecutor(runtime ToolRuntime) *ToolNodeExecutor {
	return &ToolNodeExecutor{runtime: runtime}
}

func (e *ToolNodeExecutor) ExecuteNode(ctx context.Context, request flowengine.NodeExecutionRequest) (flowengine.NodeResult, error) {
	if e.runtime == nil {
		return flowengine.NodeResult{}, apperrors.New(apperrors.CodeUnavailable, "tool runtime is not configured")
	}
	toolID := id.ID(stringConfig(request.Node.Config, "tool_id"))
	if toolID.Empty() {
		return flowengine.NodeResult{}, apperrors.New(apperrors.CodeInvalidArgument, "tool node requires tool_id")
	}

	result, err := e.runtime.Execute(ctx, tool.Call{
		ID:       id.New("wftoolcall"),
		TenantID: request.Run.TenantID,
		ToolID:   toolID,
		Name:     request.Node.Name,
		Args:     request.Input,
		TraceID:  request.Run.TraceID,
	})
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	output := map[string]any{
		"call_id":      result.CallID.String(),
		"tool_id":      result.ToolID.String(),
		"success":      result.Success,
		"data":         result.Data,
		"error_code":   result.ErrorCode,
		"error_reason": result.ErrorReason,
		"started_at":   result.StartedAt,
		"finished_at":  result.FinishedAt,
	}
	if !result.Success {
		return flowengine.NodeResult{Output: output}, apperrors.New(
			apperrors.CodeUnavailable,
			fmt.Sprintf("tool node failed: %s", firstNonEmpty(result.ErrorCode, result.ErrorReason, "unknown_error")),
		)
	}
	return flowengine.NodeResult{Output: output}, nil
}

type TransformNodeExecutor struct{}

func (TransformNodeExecutor) ExecuteNode(ctx context.Context, request flowengine.NodeExecutionRequest) (flowengine.NodeResult, error) {
	functionID := stringConfig(request.Node.Config, "function_id")
	if functionID == "" {
		// Legacy transform nodes remain useful as pure mapping nodes:
		// input_mapping prepares the node input, then output/write mappings
		// publish it to the workflow context.
		return flowengine.NodeResult{Output: request.Input}, nil
	}
	output, err := ExecuteTransformFunction(ctx, functionID, request.Input)
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	return flowengine.NodeResult{Output: output}, nil
}

type ConditionNodeExecutor struct{}

func (ConditionNodeExecutor) ExecuteNode(ctx context.Context, request flowengine.NodeExecutionRequest) (flowengine.NodeResult, error) {
	if branches := conditionBranchesFromConfig(request.Node.Config, "branches"); len(branches) > 0 {
		for index, branch := range branches {
			matched, evaluations, err := evaluateConditionBranch(branch, request)
			if err != nil {
				return flowengine.NodeResult{}, err
			}
			if !matched {
				continue
			}
			return conditionBranchResult(request, branch, index, true, evaluations)
		}
		if branch, ok := conditionBranchFromValue(request.Node.Config["default_branch"]); ok {
			return conditionBranchResult(request, branch, -1, false, nil)
		}
		return flowengine.NodeResult{
			Output: map[string]any{
				"matched":        false,
				"matched_branch": nil,
				"reason":         "no condition branch matched",
			},
			Stop: true,
		}, nil
	}

	next := id.ID(stringConfig(request.Node.Config, "next_node_id"))
	if next.Empty() {
		return flowengine.NodeResult{Output: request.Input}, nil
	}
	return flowengine.NodeResult{Output: request.Input, NextNodeIDs: []id.ID{next}}, nil
}

type conditionBranch struct {
	ID           string
	Name         string
	Mode         string
	Rules        []conditionRule
	WriteContext map[string]any
	NextNodeID   id.ID
	HasNextNode  bool
}

type conditionRule struct {
	Left     any
	Operator string
	Right    any
}

func conditionBranchesFromConfig(config map[string]any, key string) []conditionBranch {
	if config == nil {
		return nil
	}
	raw, ok := config[key].([]any)
	if !ok {
		return nil
	}
	branches := make([]conditionBranch, 0, len(raw))
	for _, item := range raw {
		branch, ok := conditionBranchFromValue(item)
		if !ok {
			continue
		}
		branches = append(branches, branch)
	}
	return branches
}

func conditionBranchFromValue(value any) (conditionBranch, bool) {
	record, ok := value.(map[string]any)
	if !ok {
		return conditionBranch{}, false
	}
	nextRaw, hasNextNode := record["next_node_id"]
	nextNodeID := id.ID(strings.TrimSpace(fmt.Sprint(nextRaw)))
	branch := conditionBranch{
		ID:           strings.TrimSpace(fmt.Sprint(record["id"])),
		Name:         strings.TrimSpace(fmt.Sprint(record["name"])),
		Mode:         strings.ToLower(strings.TrimSpace(fmt.Sprint(record["mode"]))),
		Rules:        conditionRulesFromValue(record["rules"]),
		WriteContext: conditionWriteContextFromValue(firstPresent(record, "write_context", "context_writes")),
		NextNodeID:   nextNodeID,
		HasNextNode:  hasNextNode,
	}
	if branch.Mode == "" {
		branch.Mode = "all"
	}
	return branch, true
}

func conditionRulesFromValue(value any) []conditionRule {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	rules := make([]conditionRule, 0, len(raw))
	for _, item := range raw {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rules = append(rules, conditionRule{
			Left:     firstPresent(record, "left", "path"),
			Operator: strings.ToLower(strings.TrimSpace(fmt.Sprint(firstPresent(record, "operator", "op")))),
			Right:    firstPresent(record, "right", "value"),
		})
	}
	return rules
}

func conditionWriteContextFromValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case []any:
		writes := map[string]any{}
		for _, item := range typed {
			record, ok := item.(map[string]any)
			if !ok {
				continue
			}
			target := strings.TrimSpace(fmt.Sprint(firstPresent(record, "target", "path")))
			if target == "" {
				continue
			}
			writes[target] = firstPresent(record, "source", "value")
		}
		return writes
	default:
		return nil
	}
}

func evaluateConditionBranch(branch conditionBranch, request flowengine.NodeExecutionRequest) (bool, []map[string]any, error) {
	if len(branch.Rules) == 0 {
		return true, nil, nil
	}
	evaluations := make([]map[string]any, 0, len(branch.Rules))
	matchedCount := 0
	for _, rule := range branch.Rules {
		left, err := resolveConditionValue(rule.Left, request)
		if err != nil {
			return false, nil, err
		}
		right, err := resolveConditionValue(rule.Right, request)
		if err != nil {
			return false, nil, err
		}
		operator := rule.Operator
		if operator == "" {
			operator = "equals"
		}
		matched := compareConditionValues(left, operator, right)
		if matched {
			matchedCount++
		}
		evaluations = append(evaluations, map[string]any{
			"left":     left,
			"operator": operator,
			"right":    right,
			"matched":  matched,
		})
	}
	if branch.Mode == "any" || branch.Mode == "or" {
		return matchedCount > 0, evaluations, nil
	}
	return matchedCount == len(branch.Rules), evaluations, nil
}

func conditionBranchResult(request flowengine.NodeExecutionRequest, branch conditionBranch, index int, matched bool, evaluations []map[string]any) (flowengine.NodeResult, error) {
	writes := map[string]any{}
	for target, source := range branch.WriteContext {
		value, err := resolveConditionValue(source, request)
		if err != nil {
			return flowengine.NodeResult{}, err
		}
		writes[target] = value
	}
	output := map[string]any{
		"matched":        matched,
		"matched_branch": branch.Name,
		"branch_id":      branch.ID,
		"branch_index":   index,
		"evaluations":    evaluations,
		"next_node_id":   branch.NextNodeID.String(),
		"context_writes": writes,
	}
	result := flowengine.NodeResult{Output: output, ContextWrites: writes}
	if !branch.HasNextNode || branch.NextNodeID.Empty() {
		result.Stop = true
		return result, nil
	}
	result.NextNodeIDs = []id.ID{branch.NextNodeID}
	return result, nil
}

func resolveConditionValue(source any, request flowengine.NodeExecutionRequest) (any, error) {
	if record, ok := source.(map[string]any); ok {
		valueType := strings.ToLower(strings.TrimSpace(fmt.Sprint(record["type"])))
		switch valueType {
		case "const", "constant":
			return record["value"], nil
		case "ctx", "context", "var", "variable", "expression":
			return resolveConditionValue(firstPresent(record, "path", "expression", "value"), request)
		}
	}
	text, ok := source.(string)
	if !ok {
		return source, nil
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, "$") {
		return source, nil
	}
	if strings.HasPrefix(trimmed, "$.") {
		value, ok := readMapPath(request.Input, strings.TrimPrefix(trimmed, "$."))
		if !ok {
			return nil, nil
		}
		return value, nil
	}
	value, ok := request.Context.Read(trimmed)
	if !ok {
		return nil, nil
	}
	return value, nil
}

func compareConditionValues(left any, operator string, right any) bool {
	switch operator {
	case "equals", "eq", "==":
		return valuesEqual(left, right)
	case "not_equals", "neq", "!=":
		return !valuesEqual(left, right)
	case "exists":
		return left != nil
	case "not_exists":
		return left == nil
	case "is_empty":
		return isEmptyValue(left)
	case "is_not_empty":
		return !isEmptyValue(left)
	case "contains":
		return containsValue(left, right)
	case "not_contains":
		return !containsValue(left, right)
	case "starts_with":
		return strings.HasPrefix(fmt.Sprint(left), fmt.Sprint(right))
	case "ends_with":
		return strings.HasSuffix(fmt.Sprint(left), fmt.Sprint(right))
	case "greater_than", "gt", ">":
		leftNumber, leftOK := numberValue(left)
		rightNumber, rightOK := numberValue(right)
		return leftOK && rightOK && leftNumber > rightNumber
	case "greater_or_equals", "gte", ">=":
		leftNumber, leftOK := numberValue(left)
		rightNumber, rightOK := numberValue(right)
		return leftOK && rightOK && leftNumber >= rightNumber
	case "less_than", "lt", "<":
		leftNumber, leftOK := numberValue(left)
		rightNumber, rightOK := numberValue(right)
		return leftOK && rightOK && leftNumber < rightNumber
	case "less_or_equals", "lte", "<=":
		leftNumber, leftOK := numberValue(left)
		rightNumber, rightOK := numberValue(right)
		return leftOK && rightOK && leftNumber <= rightNumber
	case "in":
		return containsValue(right, left)
	case "not_in":
		return !containsValue(right, left)
	case "regex_match":
		matched, err := regexp.MatchString(fmt.Sprint(right), fmt.Sprint(left))
		return err == nil && matched
	default:
		return valuesEqual(left, right)
	}
}

func valuesEqual(left any, right any) bool {
	if reflect.DeepEqual(left, right) {
		return true
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func containsValue(container any, item any) bool {
	switch typed := container.(type) {
	case string:
		return strings.Contains(typed, fmt.Sprint(item))
	case []any:
		for _, value := range typed {
			if valuesEqual(value, item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func readMapPath(root map[string]any, path string) (any, bool) {
	if strings.TrimSpace(path) == "" {
		return root, true
	}
	var current any = root
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = asMap[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func firstPresent(record map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := record[key]; ok {
			return value
		}
	}
	return nil
}

func stringConfig(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
