package infrastructure

import (
	"context"
	"time"

	"flow-anything/internal/platform/contracts/connector"
)

type MemoryConnectorInvoker struct{}

func NewMemoryConnectorInvoker() *MemoryConnectorInvoker {
	return &MemoryConnectorInvoker{}
}

func (i *MemoryConnectorInvoker) Invoke(ctx context.Context, req connector.InvokeRequest) (connector.InvokeResult, error) {
	return connector.InvokeResult{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]any{
			"message":      "connector invocation stub",
			"operation_id": req.OperationID.String(),
			"args":         req.Args,
		},
		FinishedAt: time.Now().UTC(),
	}, nil
}
