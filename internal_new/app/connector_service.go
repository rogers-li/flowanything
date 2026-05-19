package app

import (
	"context"

	"flow-anything/core/connector"
	"flow-anything/core/runtimecontext"
)

type ConnectorRequest struct {
	CallID       string
	OperationID  string
	Input        map[string]any
	Metadata     map[string]any
	TraceID      string
	TraceContext runtimecontext.TraceContext
}

type ConnectorResult = connector.InvokeResult

// InvokeConnector executes one connector operation through core/connector.
func (h *Host) InvokeConnector(ctx context.Context, req ConnectorRequest) (ConnectorResult, error) {
	return h.connectorRuntime.Invoke(ctx, connector.InvokeRequest{
		CallID:       req.CallID,
		OperationID:  req.OperationID,
		Input:        req.Input,
		Metadata:     req.Metadata,
		TraceID:      req.TraceID,
		TraceContext: req.TraceContext,
	})
}
