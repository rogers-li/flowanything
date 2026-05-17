package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/connector"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryConnectorRepository struct {
	mu         sync.RWMutex
	connectors map[string]connector.Spec
}

func NewMemoryConnectorRepository() *MemoryConnectorRepository {
	return &MemoryConnectorRepository{
		connectors: make(map[string]connector.Spec),
	}
}

func (r *MemoryConnectorRepository) SaveConnector(ctx context.Context, spec connector.Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.connectors[key(spec.TenantID, spec.ID)] = spec
	return nil
}

func (r *MemoryConnectorRepository) GetConnector(ctx context.Context, tenantID tenant.ID, connectorID id.ID) (connector.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.connectors[key(tenantID, connectorID)]
	if !ok {
		return connector.Spec{}, apperrors.New(apperrors.CodeNotFound, "connector not found")
	}

	return spec, nil
}

func (r *MemoryConnectorRepository) ListConnectors(ctx context.Context, tenantID tenant.ID) ([]connector.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]connector.Spec, 0)
	for _, spec := range r.connectors {
		if spec.TenantID == tenantID {
			result = append(result, spec)
		}
	}

	return result, nil
}
