package workflow

import (
	"context"

	"flow-anything/core/flowengine"
)

// Runtime is a thin application wrapper around flowengine.StatefulExecutor.
type Runtime struct {
	engine *flowengine.StatefulExecutor
}

func NewRuntime(engine *flowengine.StatefulExecutor) *Runtime {
	return &Runtime{engine: engine}
}

func (r *Runtime) Start(ctx context.Context, compiled CompiledWorkflow, input map[string]any) (flowengine.FlowInstance, error) {
	return r.engine.Start(ctx, compiled.Spec, input)
}

func (r *Runtime) StartWithContext(ctx context.Context, compiled CompiledWorkflow, input map[string]any, initialContext *flowengine.DataContext) (flowengine.FlowInstance, error) {
	return r.engine.StartWithContext(ctx, compiled.Spec, input, initialContext)
}

func (r *Runtime) Resume(ctx context.Context, compiled CompiledWorkflow, instanceID string, event flowengine.ExternalEvent) (flowengine.FlowInstance, error) {
	return r.engine.Resume(ctx, compiled.Spec, instanceID, event)
}
