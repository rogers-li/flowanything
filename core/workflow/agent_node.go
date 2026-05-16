package workflow

import (
	"context"
	"fmt"

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
		TraceContext: traceContextFrom(ctx),
	})
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	output := result.Output
	if output == nil {
		output = map[string]any{}
	}
	if result.Text != "" {
		output["text"] = result.Text
	}
	if req.Context != nil {
		_ = req.Context.WriteNodeContext("agent", "runs."+config.Agent.ID, map[string]any{
			"output": output,
			"text":   result.Text,
			"raw":    result.Raw,
		})
	}
	return flowengine.NodeResult{Output: output, NextNodeIDs: result.NextNodeIDs}, nil
}
