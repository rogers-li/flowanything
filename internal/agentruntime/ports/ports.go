package ports

import (
	"context"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type ToolAdapter interface {
	Supports(kind tool.ImplementationType) bool
	Execute(ctx context.Context, spec tool.Spec, call tool.Call) (tool.Result, error)
}

type ToolCatalog interface {
	GetTool(ctx context.Context, call tool.Call) (tool.Spec, error)
}

type ExecutionRecorder interface {
	RecordStarted(ctx context.Context, record tool.ExecutionRecord) error
	RecordFinished(ctx context.Context, record tool.ExecutionRecord) error
}

type ExecutionReader interface {
	GetExecution(ctx context.Context, tenantID tenant.ID, callID id.ID) (tool.ExecutionRecord, error)
}
