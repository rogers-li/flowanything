package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/agentflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryAgentFlowRepository struct {
	mu    sync.RWMutex
	flows map[string]agentflow.Spec
}

func NewMemoryAgentFlowRepository() *MemoryAgentFlowRepository {
	return &MemoryAgentFlowRepository{
		flows: map[string]agentflow.Spec{},
	}
}

func (r *MemoryAgentFlowRepository) SaveAgentFlow(ctx context.Context, spec agentflow.Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.flows[key(spec.TenantID, spec.ID)] = spec
	return nil
}

func (r *MemoryAgentFlowRepository) GetAgentFlow(ctx context.Context, tenantID tenant.ID, flowID id.ID) (agentflow.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.flows[key(tenantID, flowID)]
	if !ok {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeNotFound, "agent flow not found")
	}
	return spec, nil
}

func (r *MemoryAgentFlowRepository) ListAgentFlows(ctx context.Context, tenantID tenant.ID) ([]agentflow.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]agentflow.Spec, 0)
	for _, spec := range r.flows {
		if spec.TenantID == tenantID {
			result = append(result, spec)
		}
	}
	return result, nil
}
