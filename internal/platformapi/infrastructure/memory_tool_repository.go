package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryToolRepository struct {
	mu    sync.RWMutex
	tools map[string]tool.Spec
}

func NewMemoryToolRepository() *MemoryToolRepository {
	return &MemoryToolRepository{
		tools: make(map[string]tool.Spec),
	}
}

func (r *MemoryToolRepository) SaveTool(ctx context.Context, spec tool.Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[key(spec.TenantID, spec.ID)] = spec
	return nil
}

func (r *MemoryToolRepository) GetTool(ctx context.Context, tenantID tenant.ID, toolID id.ID) (tool.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.tools[key(tenantID, toolID)]
	if !ok {
		return tool.Spec{}, apperrors.New(apperrors.CodeNotFound, "tool not found")
	}

	return spec, nil
}

func (r *MemoryToolRepository) ListTools(ctx context.Context, tenantID tenant.ID) ([]tool.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]tool.Spec, 0)
	for _, spec := range r.tools {
		if spec.TenantID == tenantID {
			result = append(result, spec)
		}
	}

	return result, nil
}
