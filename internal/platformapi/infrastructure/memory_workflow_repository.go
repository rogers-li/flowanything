package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryWorkflowRepository struct {
	mu        sync.RWMutex
	workflows map[string]workflow.Spec
}

func NewMemoryWorkflowRepository() *MemoryWorkflowRepository {
	return &MemoryWorkflowRepository{workflows: map[string]workflow.Spec{}}
}

func (r *MemoryWorkflowRepository) SaveWorkflow(ctx context.Context, spec workflow.Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflows[key(spec.TenantID, spec.ID)] = spec
	return nil
}

func (r *MemoryWorkflowRepository) GetWorkflow(ctx context.Context, tenantID tenant.ID, workflowID id.ID) (workflow.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.workflows[key(tenantID, workflowID)]
	if !ok {
		return workflow.Spec{}, apperrors.New(apperrors.CodeNotFound, "workflow not found")
	}
	return spec, nil
}

func (r *MemoryWorkflowRepository) ListWorkflows(ctx context.Context, tenantID tenant.ID) ([]workflow.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]workflow.Spec, 0)
	for _, spec := range r.workflows {
		if spec.TenantID == tenantID {
			result = append(result, spec)
		}
	}
	return result, nil
}
