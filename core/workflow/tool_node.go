package workflow

import (
	"context"
	"fmt"

	"flow-anything/core/flowengine"
)

type ToolNodeConfig struct {
	ToolID   string         `json:"tool_id"`
	Metadata map[string]any `json:"metadata"`
}

type ToolNodeExecutor struct {
	invoker ToolInvoker
}

func NewToolNodeExecutor(invoker ToolInvoker) ToolNodeExecutor {
	return ToolNodeExecutor{invoker: invoker}
}

func (e ToolNodeExecutor) Type() string { return NodeTypeTool }

func (e ToolNodeExecutor) Validate(_ context.Context, node flowengine.NodeSpec) error {
	config, err := decodeNodeConfig[ToolNodeConfig](node.Config)
	if err != nil {
		return err
	}
	if config.ToolID == "" {
		return fmt.Errorf("tool_id is required")
	}
	return nil
}

func (e ToolNodeExecutor) Execute(ctx context.Context, req flowengine.NodeRequest) (flowengine.NodeResult, error) {
	if e.invoker == nil {
		return flowengine.NodeResult{}, fmt.Errorf("tool invoker is not configured")
	}
	config, err := decodeNodeConfig[ToolNodeConfig](req.Node.Config)
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	result, err := e.invoker.InvokeTool(ctx, ToolInvokeRequest{
		ToolID:       config.ToolID,
		Input:        req.Input,
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
	if req.Context != nil {
		_ = req.Context.WriteNodeContext("tool", "calls."+config.ToolID, map[string]any{
			"output": output,
			"raw":    result.Raw,
		})
	}
	return flowengine.NodeResult{Output: output}, nil
}
