package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/connector"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryConnectorOperationRepository struct {
	mu         sync.RWMutex
	operations map[string]connector.OperationSpec
}

func NewMemoryConnectorOperationRepository() *MemoryConnectorOperationRepository {
	return &MemoryConnectorOperationRepository{
		operations: make(map[string]connector.OperationSpec),
	}
}

func (r *MemoryConnectorOperationRepository) SaveConnectorOperation(ctx context.Context, spec connector.OperationSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.operations[key(spec.TenantID, spec.ID)] = spec
	return nil
}

func (r *MemoryConnectorOperationRepository) GetConnectorOperation(ctx context.Context, tenantID tenant.ID, operationID id.ID) (connector.OperationSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.operations[key(tenantID, operationID)]
	if !ok {
		return connector.OperationSpec{}, apperrors.New(apperrors.CodeNotFound, "connector operation not found")
	}

	return spec, nil
}

func (r *MemoryConnectorOperationRepository) ListConnectorOperations(ctx context.Context, tenantID tenant.ID) ([]connector.OperationSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]connector.OperationSpec, 0)
	for _, spec := range r.operations {
		if spec.TenantID == tenantID {
			result = append(result, spec)
		}
	}

	return result, nil
}
