package workflow

import (
	"context"
	"fmt"
	"strings"

	"flow-anything/core/agentcore"
	"flow-anything/core/flowengine"
)

type AgentNodeConfig struct {
	Agent        agentcore.AgentSpec `json:"agent"`
	MessageField string              `json:"message_field"`
	Metadata     map[string]any      `json:"metadata"`
}

type AgentNodeExecutor struct {
	runner AgentRunner
}

func NewAgentNodeExecutor(runner AgentRunner) AgentNodeExecutor {
	return AgentNodeExecutor{runner: runner}
}

func (e AgentNodeExecutor) Type() string { return NodeTypeAgent }

func (e AgentNodeExecutor) Validate(_ context.Context, node flowengine.NodeSpec) error {
	config, err := decodeNodeConfig[AgentNodeConfig](node.Config)
	if err != nil {
		return err
	}
	if config.Agent.ID == "" {
		return fmt.Errorf("agent.id is required")
	}
	return nil
}

func (e AgentNodeExecutor) Execute(ctx context.Context, req flowengine.NodeRequest) (flowengine.NodeResult, error) {
	if e.runner == nil {
		return flowengine.NodeResult{}, fmt.Errorf("agent runner is not configured")
	}
	config, err := decodeNodeConfig[AgentNodeConfig](req.Node.Config)
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	message := ""
	if config.MessageField != "" {
		message = fmt.Sprint(readNested(req.Input, config.MessageField))
	}
	result, err := e.runner.RunAgent(ctx, AgentRunRequest{
		Agent:        config.Agent,
		Input:        req.Input,
		Message:      message,
		Metadata:     config.Metadata,
		TraceContext: childTraceContextFrom(ctx),
	})
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	output := result.Output
	if output == nil {
		output = map[string]any{}
	}
	text := agentOutputText(output, result.Text)
	if text != "" {
		output["text"] = text
	}
	if req.Context != nil {
		_ = req.Context.WriteNodeContext("agent", "runs."+config.Agent.ID, map[string]any{
			"output": output,
			"text":   text,
			"raw":    result.Raw,
		})
	}
	nextNodeIDs := result.NextNodeIDs
	if agentRoutingMode(config.Metadata) == "agent_directed" {
		nextNodeIDs = selectedNextNodeIDs(req.Flow, req.Node.ID, output["next_node_ids"])
	}
	return flowengine.NodeResult{Output: output, NextNodeIDs: nextNodeIDs}, nil
}

func agentOutputText(output map[string]any, fallback string) string {
	for _, key := range []string{"text", "return_message", "answer", "message", "final_answer", "result"} {
		if value, ok := output[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return fallback
}

func agentRoutingMode(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(metadata["agent_routing_mode"]))
}

func selectedNextNodeIDs(flow flowengine.FlowSpec, nodeID string, value any) []string {
	allowed := map[string]struct{}{}
	for _, edge := range flow.Edges {
		if edge.From == nodeID && edge.To != "" {
			allowed[edge.To] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return []string{}
	}
	selected := []string{}
	for _, id := range stringListValue(value) {
		if _, ok := allowed[id]; ok {
			selected = append(selected, id)
		}
	}
	return selected
}

func stringListValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
