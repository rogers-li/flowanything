package ports

import (
	"context"

	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type OperationRepository interface {
	GetOperation(ctx context.Context, tenantID tenant.ID, operationID id.ID) (connector.OperationSpec, error)
}

type OperationInvoker interface {
	Invoke(ctx context.Context, operation connector.OperationSpec, req connector.InvokeRequest) (connector.InvokeResult, error)
}
