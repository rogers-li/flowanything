package flowengine

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	NodeTypeStart     = "control.start"
	NodeTypeEnd       = "control.end"
	NodeTypeNoop      = "control.noop"
	NodeTypeCondition = "control.condition"
	NodeTypeWait      = "control.wait"
	NodeTypeParallel  = "control.parallel"
	NodeTypeJoin      = "control.join"
)

// RegisterControlNodes registers built-in control-flow nodes.
func RegisterControlNodes(registry *Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	for _, executor := range []NodeExecutor{
		ControlNodeExecutor{NodeType: NodeTypeStart},
		ControlNodeExecutor{NodeType: NodeTypeNoop},
		ControlNodeExecutor{NodeType: NodeTypeEnd},
		ControlNodeExecutor{NodeType: NodeTypeWait},
		ControlNodeExecutor{NodeType: NodeTypeParallel},
		ControlNodeExecutor{NodeType: NodeTypeJoin},
		ConditionNodeExecutor{},
	} {
		if err := registry.Register(executor); err != nil {
			return err
		}
	}
	return nil
}

// NewDefaultRegistry creates a registry with flow-engine control nodes already
// installed. Product modules can then register business node executors on top.
func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = RegisterControlNodes(registry)
	return registry
}

// ControlNodeExecutor implements start/noop/end nodes.
type ControlNodeExecutor struct {
	NodeType string
}

func (e ControlNodeExecutor) Type() string { return e.NodeType }

func (e ControlNodeExecutor) Validate(context.Context, NodeSpec) error {
	switch e.NodeType {
	case NodeTypeStart, NodeTypeNoop, NodeTypeEnd, NodeTypeWait, NodeTypeParallel, NodeTypeJoin:
		return nil
	default:
		return fmt.Errorf("unsupported control node type %q", e.NodeType)
	}
}

func (e ControlNodeExecutor) Execute(_ context.Context, req NodeRequest) (NodeResult, error) {
	output := map[string]any{
		"node_id":   req.Node.ID,
		"node_type": req.Node.Type,
	}
	if e.NodeType == NodeTypeEnd {
		return NodeResult{Output: output, NextNodeIDs: []string{}}, nil
	}
	if e.NodeType == NodeTypeWait {
		return NodeResult{}, fmt.Errorf("wait node requires stateful executor")
	}
	return NodeResult{Output: output}, nil
}

// ConditionNodeExecutor routes control flow based on branch rules.
type ConditionNodeExecutor struct{}

func (ConditionNodeExecutor) Type() string { return NodeTypeCondition }

func (ConditionNodeExecutor) Validate(_ context.Context, node NodeSpec) error {
	config, err := decodeConditionConfig(node.Config)
	if err != nil {
		return err
	}
	if len(config.Branches) == 0 && len(config.DefaultNextNodeIDs) == 0 {
		return fmt.Errorf("condition node requires at least one branch or default next node")
	}
	for branchIndex, branch := range config.Branches {
		if branch.Name == "" {
			return fmt.Errorf("condition branch[%d] name is required", branchIndex)
		}
		mode := branch.Mode
		if mode == "" {
			mode = ConditionModeAll
		}
		if mode != ConditionModeAll && mode != ConditionModeAny {
			return fmt.Errorf("condition branch %q has unsupported mode %q", branch.Name, branch.Mode)
		}
		for ruleIndex, rule := range branch.Rules {
			if rule.Operator == "" {
				return fmt.Errorf("condition branch %q rule[%d] operator is required", branch.Name, ruleIndex)
			}
		}
	}
	return nil
}

func (ConditionNodeExecutor) Execute(_ context.Context, req NodeRequest) (NodeResult, error) {
	config, err := decodeConditionConfig(req.Node.Config)
	if err != nil {
		return NodeResult{}, err
	}

	for _, branch := range config.Branches {
		matched, err := evaluateConditionBranch(req.Context, branch)
		if err != nil {
			return NodeResult{}, fmt.Errorf("evaluate branch %q: %w", branch.Name, err)
		}
		if !matched {
			continue
		}
		output := conditionOutput(true, branch.Name, branch.Output)
		if err := ApplyContextWrites(req.Context, output, branch.ContextWrites); err != nil {
			return NodeResult{}, fmt.Errorf("apply branch %q context writes: %w", branch.Name, err)
		}
		return NodeResult{Output: output, NextNodeIDs: normalizeNextNodeIDs(branch.NextNodeIDs)}, nil
	}

	output := conditionOutput(false, "", config.DefaultOutput)
	if err := ApplyContextWrites(req.Context, output, config.DefaultWrites); err != nil {
		return NodeResult{}, fmt.Errorf("apply default branch context writes: %w", err)
	}
	return NodeResult{Output: output, NextNodeIDs: normalizeNextNodeIDs(config.DefaultNextNodeIDs)}, nil
}

// ConditionConfig is the built-in condition node protocol.
type ConditionConfig struct {
	Branches           []ConditionBranch `json:"branches"`
	DefaultNextNodeIDs []string          `json:"default_next_node_ids"`
	DefaultOutput      map[string]any    `json:"default_output"`
	DefaultWrites      []ContextWrite    `json:"default_writes"`
}

type ConditionBranch struct {
	Name          string          `json:"name"`
	Mode          ConditionMode   `json:"mode"`
	Rules         []ConditionRule `json:"rules"`
	NextNodeIDs   []string        `json:"next_node_ids"`
	Output        map[string]any  `json:"output"`
	ContextWrites []ContextWrite  `json:"context_writes"`
}

type ConditionMode string

const (
	ConditionModeAll ConditionMode = "all"
	ConditionModeAny ConditionMode = "any"
)

type ConditionRule struct {
	Left     ValueSource       `json:"left"`
	Operator ConditionOperator `json:"operator"`
	Right    ValueSource       `json:"right"`
	Negate   bool              `json:"negate"`
}

type ConditionOperator string

const (
	ConditionOpExists    ConditionOperator = "exists"
	ConditionOpNotExists ConditionOperator = "not_exists"
	ConditionOpEmpty     ConditionOperator = "empty"
	ConditionOpNotEmpty  ConditionOperator = "not_empty"
	ConditionOpEquals    ConditionOperator = "eq"
	ConditionOpNotEquals ConditionOperator = "neq"
	ConditionOpGreater   ConditionOperator = "gt"
	ConditionOpGreaterEq ConditionOperator = "gte"
	ConditionOpLess      ConditionOperator = "lt"
	ConditionOpLessEq    ConditionOperator = "lte"
	ConditionOpContains  ConditionOperator = "contains"
	ConditionOpIn        ConditionOperator = "in"
)

func decodeConditionConfig(config map[string]any) (ConditionConfig, error) {
	if config == nil {
		return ConditionConfig{}, fmt.Errorf("condition config is required")
	}
	data, err := json.Marshal(config)
	if err != nil {
		return ConditionConfig{}, err
	}
	var decoded ConditionConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		return ConditionConfig{}, err
	}
	return decoded, nil
}

func evaluateConditionBranch(data *DataContext, branch ConditionBranch) (bool, error) {
	if len(branch.Rules) == 0 {
		return true, nil
	}
	mode := branch.Mode
	if mode == "" {
		mode = ConditionModeAll
	}
	matchedAny := false
	for _, rule := range branch.Rules {
		matched, err := evaluateConditionRule(data, rule)
		if err != nil {
			return false, err
		}
		if rule.Negate {
			matched = !matched
		}
		if mode == ConditionModeAny && matched {
			return true, nil
		}
		if mode == ConditionModeAll && !matched {
			return false, nil
		}
		matchedAny = matchedAny || matched
	}
	return matchedAny, nil
}

func evaluateConditionRule(data *DataContext, rule ConditionRule) (bool, error) {
	left, leftFound, err := resolveConditionValue(data, rule.Left)
	if err != nil {
		return false, err
	}
	switch rule.Operator {
	case ConditionOpExists:
		return leftFound, nil
	case ConditionOpNotExists:
		return !leftFound, nil
	case ConditionOpEmpty:
		return isEmptyValue(left), nil
	case ConditionOpNotEmpty:
		return !isEmptyValue(left), nil
	}
	if !leftFound {
		return false, nil
	}
	right, _, err := resolveConditionValue(data, rule.Right)
	if err != nil {
		return false, err
	}
	switch rule.Operator {
	case ConditionOpEquals:
		return valuesEqual(left, right), nil
	case ConditionOpNotEquals:
		return !valuesEqual(left, right), nil
	case ConditionOpGreater, ConditionOpGreaterEq, ConditionOpLess, ConditionOpLessEq:
		return compareOrdered(left, right, rule.Operator)
	case ConditionOpContains:
		return containsValue(left, right), nil
	case ConditionOpIn:
		return containsValue(right, left), nil
	default:
		return false, fmt.Errorf("unsupported condition operator %q", rule.Operator)
	}
}

func resolveConditionValue(data *DataContext, source ValueSource) (any, bool, error) {
	if source.Type == "" {
		if source.Path != "" {
			source.Type = SourceContext
		} else {
			source.Type = SourceConst
		}
	}
	switch source.Type {
	case SourceConst:
		return source.Value, true, nil
	case SourceContext:
		value, ok := data.Read(source.Path)
		return value, ok, nil
	default:
		value, err := ResolveValue(data, nil, source)
		return value, err == nil, err
	}
}

func conditionOutput(matched bool, branchName string, extra map[string]any) map[string]any {
	output := map[string]any{
		"matched": matched,
		"branch":  branchName,
	}
	for key, value := range extra {
		output[key] = value
	}
	return output
}

func normalizeNextNodeIDs(nextNodeIDs []string) []string {
	if nextNodeIDs == nil {
		return []string{}
	}
	return nextNodeIDs
}

func valuesEqual(left, right any) bool {
	if leftNumber, ok := numberValue(left); ok {
		if rightNumber, ok := numberValue(right); ok {
			return leftNumber == rightNumber
		}
	}
	return reflect.DeepEqual(left, right)
}

func compareOrdered(left, right any, operator ConditionOperator) (bool, error) {
	leftNumber, leftOK := numberValue(left)
	rightNumber, rightOK := numberValue(right)
	if leftOK && rightOK {
		switch operator {
		case ConditionOpGreater:
			return leftNumber > rightNumber, nil
		case ConditionOpGreaterEq:
			return leftNumber >= rightNumber, nil
		case ConditionOpLess:
			return leftNumber < rightNumber, nil
		case ConditionOpLessEq:
			return leftNumber <= rightNumber, nil
		}
	}
	leftString, leftOK := left.(string)
	rightString, rightOK := right.(string)
	if leftOK && rightOK {
		switch operator {
		case ConditionOpGreater:
			return leftString > rightString, nil
		case ConditionOpGreaterEq:
			return leftString >= rightString, nil
		case ConditionOpLess:
			return leftString < rightString, nil
		case ConditionOpLessEq:
			return leftString <= rightString, nil
		}
	}
	return false, fmt.Errorf("operator %q requires comparable numbers or strings", operator)
}

func containsValue(container, value any) bool {
	switch typed := container.(type) {
	case string:
		return strings.Contains(typed, fmt.Sprint(value))
	case []any:
		for _, item := range typed {
			if valuesEqual(item, value) {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if valuesEqual(item, value) {
				return true
			}
		}
	}
	reflected := reflect.ValueOf(container)
	if reflected.Kind() == reflect.Slice || reflected.Kind() == reflect.Array {
		for i := 0; i < reflected.Len(); i++ {
			if valuesEqual(reflected.Index(i).Interface(), value) {
				return true
			}
		}
	}
	return false
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		return reflected.Len() == 0
	case reflect.Bool:
		return !reflected.Bool()
	}
	return false
}
