package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryToolCatalog struct {
	mu    sync.RWMutex
	tools map[string]tool.Spec
}

func NewMemoryToolCatalog(specs []tool.Spec) *MemoryToolCatalog {
	catalog := &MemoryToolCatalog{
		tools: make(map[string]tool.Spec),
	}
	for _, spec := range specs {
		catalog.tools[key(spec.TenantID, spec.ID)] = spec
	}

	return catalog
}

func (c *MemoryToolCatalog) GetTool(ctx context.Context, call tool.Call) (tool.Spec, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if call.ToolID.Empty() {
		return tool.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tool_id is required")
	}
	if call.TenantID.Empty() {
		return tool.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}

	spec, ok := c.tools[key(call.TenantID, call.ToolID)]
	if !ok {
		return tool.Spec{}, apperrors.New(apperrors.CodeNotFound, "tool not found")
	}

	return spec, nil
}

func key(tenantID tenant.ID, toolID id.ID) string {
	return tenantID.String() + "/" + toolID.String()
}
