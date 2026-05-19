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
		TraceContext: childTraceContextFrom(ctx),
	})
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	output := connectorNodeOutput(result)
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

func connectorNodeOutput(result ConnectorInvokeResult) map[string]any {
	output := copyMap(result.Output)
	if !isHTTPEnvelope(output) {
		return output
	}
	body, ok := output["body"].(map[string]any)
	if !ok {
		return output
	}
	normalized := copyMap(body)
	normalized["_connector"] = connectorEnvelope(output, result.Raw)
	return normalized
}

func isHTTPEnvelope(output map[string]any) bool {
	if output == nil {
		return false
	}
	_, hasBody := output["body"]
	_, hasStatusCode := output["status_code"]
	_, hasSuccess := output["success"]
	return hasBody && (hasStatusCode || hasSuccess)
}

func connectorEnvelope(output map[string]any, raw any) map[string]any {
	envelope := copyMap(output)
	if raw != nil {
		envelope["raw"] = raw
	}
	return envelope
}

func copyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func childTraceContextFrom(ctx context.Context) runtimecontext.TraceContext {
	traceContext, _ := runtimecontext.TraceContextFrom(ctx)
	return runtimecontext.TraceContext{
		TraceID:       traceContext.TraceID,
		ParentSpanID:  traceContext.SpanID,
		CorrelationID: traceContext.CorrelationID,
	}
}
