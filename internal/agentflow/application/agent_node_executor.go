package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/platform/contracts/event"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type AgentNodeExecutor struct {
	invoker ports.AgentInvoker
}

func NewAgentNodeExecutor(invoker ports.AgentInvoker) *AgentNodeExecutor {
	return &AgentNodeExecutor{invoker: invoker}
}

func (e *AgentNodeExecutor) ExecuteNode(ctx context.Context, request ports.NodeExecutionRequest) (domain.NodeResult, error) {
	if e.invoker == nil {
		return domain.NodeResult{}, apperrors.New(apperrors.CodeInvalidArgument, "agent invoker is not configured")
	}

	config, err := parseAgentNodeConfig(request.Node, request.Context)
	if err != nil {
		return domain.NodeResult{}, err
	}

	result, err := e.invoker.InvokeAgent(ctx, ports.AgentInvocationRequest{
		Run:                 request.Run,
		Node:                request.Node,
		Context:             request.Context,
		AgentID:             config.agentID,
		RuntimeSystemPrompt: config.runtimeSystemPrompt,
		Task:                config.task,
		Payload:             config.payload,
		TraceID:             config.traceID,
		SessionID:           config.sessionID,
		UserID:              config.userID,
	})
	if err != nil {
		return domain.NodeResult{}, err
	}

	text := agentResponseText(result)
	traceID := result.TraceID
	if traceID == "" {
		traceID = result.Response.TraceID
	}

	output := map[string]any{
		"agent_id": config.agentID.String(),
		"text":     text,
		"trace_id": traceID,
		"actions":  agentActions(result),
		"response": result.Response,
	}
	if config.parseJSONOutput {
		structured, parseErr := parseJSONOutput(text)
		if parseErr != nil {
			return domain.NodeResult{}, parseErr
		}
		output["structured"] = structured
		for key, value := range structured {
			output[key] = value
		}
	}
	for key, value := range result.Raw {
		output[key] = value
	}
	nextNodeIDs := []id.ID(nil)
	if config.agentDirectedRouting {
		decision := parseAgentRoutingDecision(output, text)
		output["routing_decision"] = decision.metadata
		nextNodeIDs = decision.nextNodeIDs
	}

	variables := map[string]any{
		config.outputKey:               text,
		config.outputKey + "_trace_id": traceID,
	}

	return domain.NodeResult{Output: output, Variables: variables, NextNodeIDs: nextNodeIDs}, nil
}

type agentNodeConfig struct {
	agentID              id.ID
	task                 string
	payload              map[string]any
	outputKey            string
	parseJSONOutput      bool
	agentDirectedRouting bool
	runtimeSystemPrompt  string
	traceID              string
	sessionID            id.ID
	userID               string
}

func parseAgentNodeConfig(node domain.Node, ctx domain.RunContext) (agentNodeConfig, error) {
	agentID := id.ID(stringConfig(node.Config, "agent_id"))
	if agentID.Empty() {
		return agentNodeConfig{}, apperrors.New(apperrors.CodeInvalidArgument, "agent node requires agent_id")
	}

	task, err := resolveAgentTask(node.Config, ctx)
	if err != nil {
		return agentNodeConfig{}, err
	}

	outputKey := stringConfig(node.Config, "output_key")
	if outputKey == "" {
		outputKey = node.ID.String() + "_response"
	}

	return agentNodeConfig{
		agentID:   agentID,
		task:      task,
		payload:   mapConfig(node.Config, "payload"),
		outputKey: outputKey,
		parseJSONOutput: boolConfig(node.Config, "parse_json_output") ||
			strings.EqualFold(stringConfig(node.Config, "output_mode"), "json") ||
			hasOutputSchemaFields(node.Config),
		agentDirectedRouting: agentRoutingEnabled(node.Config),
		runtimeSystemPrompt:  stringConfig(node.Config, "runtime_system_prompt"),
		traceID:              stringConfig(node.Config, "trace_id"),
		sessionID:            id.ID(stringConfig(node.Config, "session_id")),
		userID:               stringConfig(node.Config, "user_id"),
	}, nil
}

func resolveAgentTask(config map[string]any, ctx domain.RunContext) (string, error) {
	if path := stringConfig(config, "task_path"); path != "" {
		value, ok := ctx.ValueAtPath(path)
		if !ok {
			return "", apperrors.New(apperrors.CodeInvalidArgument, "agent node task_path did not resolve")
		}
		return requireText(value, "agent node task_path resolved to empty text")
	}
	if task := stringConfig(config, "task"); task != "" {
		return task, nil
	}

	if len(ctx.Input) > 0 {
		return buildStructuredAgentTask(config, ctx.Input), nil
	}

	return "", apperrors.New(apperrors.CodeInvalidArgument, "agent node requires task, task_path, or non-empty input")
}

func buildStructuredAgentTask(config map[string]any, input map[string]any) string {
	var builder strings.Builder
	builder.WriteString("当前 Agent Node 的输入如下。请根据 Input Contract 理解每个字段的业务含义，并完成当前节点任务。\n\n")

	builder.WriteString("Input Contract:\n")
	fields := schemaFieldsFromSchema(config["input_schema"])
	if len(fields) > 0 {
		for _, field := range fields {
			line := "- " + field.Path + ": " + nonEmptyString(field.Type, "any")
			if field.Required {
				line += " required"
			}
			if strings.TrimSpace(field.Description) != "" {
				line += " - " + strings.TrimSpace(field.Description)
			}
			writeRuntimePromptLine(&builder, line)
		}
	} else {
		for _, key := range sortedMapKeys(input) {
			writeRuntimePromptLine(&builder, fmt.Sprintf("- %s: %s", key, valueTypeName(input[key])))
		}
	}

	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		payload = []byte(fmt.Sprint(input))
	}
	builder.WriteString("\nCurrent Input:\n")
	builder.Write(payload)
	return builder.String()
}

func agentResponseText(result ports.AgentInvocationResult) string {
	if strings.TrimSpace(result.Text) != "" {
		return result.Text
	}

	actions := agentActions(result)
	for _, preferred := range []event.ActionType{
		event.ActionSpeak,
		event.ActionDisplayText,
		event.ActionAskQuestion,
		event.ActionAskConfirmation,
	} {
		for _, action := range actions {
			if action.Type == preferred && strings.TrimSpace(action.Text) != "" {
				return action.Text
			}
		}
	}
	for _, action := range actions {
		if strings.TrimSpace(action.Text) != "" {
			return action.Text
		}
	}
	return ""
}

func agentActions(result ports.AgentInvocationResult) []event.Action {
	if len(result.Actions) > 0 {
		return result.Actions
	}
	return result.Response.Actions
}

func stringConfig(config map[string]any, key string) string {
	value, ok := config[key]
	if !ok {
		return ""
	}
	text, _ := requireText(value, "")
	return text
}

func mapConfig(config map[string]any, key string) map[string]any {
	value, ok := config[key]
	if !ok {
		return map[string]any{}
	}
	typed, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	result := make(map[string]any, len(typed))
	for k, v := range typed {
		result[k] = v
	}
	return result
}

func boolConfig(config map[string]any, key string) bool {
	value, ok := config[key]
	if !ok {
		return false
	}
	typed, ok := value.(bool)
	return ok && typed
}

func agentRoutingEnabled(config map[string]any) bool {
	if boolConfig(config, "agent_directed_routing") {
		return true
	}
	mode := strings.ToLower(stringConfig(config, "agent_routing_mode"))
	return mode == "agent_directed" || mode == "agent-directed"
}

func hasOutputSchemaFields(config map[string]any) bool {
	schema := mapConfig(config, "output_schema")
	rawFields, ok := schema["x-flow-fields"].([]any)
	if ok && len(rawFields) > 0 {
		return true
	}
	properties, ok := schema["properties"].(map[string]any)
	return ok && len(properties) > 0
}

func requireText(value any, message string) (string, error) {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		if message == "" {
			message = "text is empty"
		}
		return "", apperrors.New(apperrors.CodeInvalidArgument, message)
	}
	return text, nil
}

func parseJSONOutput(text string) (map[string]any, error) {
	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInvalidArgument, "agent node output is not valid JSON", err)
	}
	return result, nil
}

type agentRoutingDecision struct {
	nextNodeIDs []id.ID
	metadata    map[string]any
}

func parseAgentRoutingDecision(output map[string]any, text string) agentRoutingDecision {
	source := output
	if len(source) == 0 {
		source = map[string]any{}
	}
	if len(source) == 0 || source["next_node_ids"] == nil && source["next_node_id"] == nil && source["next_nodes"] == nil {
		if parsed, err := parseJSONOutput(text); err == nil {
			source = parsed
		}
	}

	nextIDs := extractNextNodeIDs(source)
	reason := stringFromAny(source["reason"])
	if reason == "" {
		if routing, ok := source["routing"].(map[string]any); ok {
			reason = stringFromAny(routing["reason"])
		}
	}
	metadata := map[string]any{
		"mode":          "agent_directed",
		"next_node_ids": idStrings(nextIDs),
		"accepted":      len(nextIDs) > 0,
	}
	if reason != "" {
		metadata["reason"] = reason
	}
	if len(nextIDs) == 0 {
		metadata["fallback"] = "flow_edges"
	}
	return agentRoutingDecision{nextNodeIDs: nextIDs, metadata: metadata}
}

func extractNextNodeIDs(source map[string]any) []id.ID {
	if nested, ok := source["routing"].(map[string]any); ok {
		if ids := idsFromAny(firstPresent(nested, "next_node_ids", "next_node_id", "next_nodes")); len(ids) > 0 {
			return ids
		}
	}
	return idsFromAny(firstPresent(source, "next_node_ids", "next_node_id", "next_nodes"))
}

func firstPresent(source map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := source[key]; ok {
			return value
		}
	}
	return nil
}

func idsFromAny(value any) []id.ID {
	switch typed := value.(type) {
	case []string:
		result := make([]id.ID, 0, len(typed))
		for _, item := range typed {
			if clean := strings.TrimSpace(item); clean != "" {
				result = append(result, id.ID(clean))
			}
		}
		return result
	case []any:
		result := make([]id.ID, 0, len(typed))
		for _, item := range typed {
			if clean := strings.TrimSpace(fmt.Sprint(item)); clean != "" && clean != "<nil>" {
				result = append(result, id.ID(clean))
			}
		}
		return result
	case string:
		if clean := strings.TrimSpace(typed); clean != "" {
			return []id.ID{id.ID(clean)}
		}
	}
	return nil
}

func idStrings(ids []id.ID) []string {
	result := make([]string, 0, len(ids))
	for _, item := range ids {
		result = append(result, item.String())
	}
	return result
}

func stringFromAny(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}
