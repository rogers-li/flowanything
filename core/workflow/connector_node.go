package workflow

import (
	"context"
	"fmt"

	"flow-anything/core/flowengine"
	"flow-anything/core/runtimecontext"
)

type ConnectorNodeConfig struct {
	OperationID string         `json:"operation_id"`
	Metadata    map[string]any `json:"metadata"`
}

type ConnectorNodeExecutor struct {
	invoker ConnectorInvoker
}

func NewConnectorNodeExecutor(invoker ConnectorInvoker) ConnectorNodeExecutor {
	return ConnectorNodeExecutor{invoker: invoker}
}

func (e ConnectorNodeExecutor) Type() string { return NodeTypeConnector }

func (e ConnectorNodeExecutor) Validate(_ context.Context, node flowengine.NodeSpec) error {
	config, err := decodeNodeConfig[ConnectorNodeConfig](node.Config)
	if err != nil {
		return err
	}
	if config.OperationID == "" {
		return fmt.Errorf("connector operation_id is required")
	}
	return nil
}

func (e ConnectorNodeExecutor) Execute(ctx context.Context, req flowengine.NodeRequest) (flowengine.NodeResult, error) {
	if e.invoker == nil {
		return flowengine.NodeResult{}, fmt.Errorf("connector invoker is not configured")
	}
	config, err := decodeNodeConfig[ConnectorNodeConfig](req.Node.Config)
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	result, err := e.invoker.InvokeConnector(ctx, ConnectorInvokeRequest{
		OperationID:  config.OperationID,
		Input:        req.Input,
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
	if req.Context != nil {
		_ = req.Context.WriteNodeContext("connector", "responses."+config.OperationID, map[string]any{
			"output": output,
			"raw":    result.Raw,
		})
	}
	return flowengine.NodeResult{Output: output}, nil
}

func traceContextFrom(ctx context.Context) runtimecontext.TraceContext {
	traceContext, _ := runtimecontext.TraceContextFrom(ctx)
	return traceContext
}
