package workflowruntime

import (
	"context"

	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type WorkflowLoader interface {
	LoadWorkflow(ctx context.Context, tenantID tenant.ID, workflowID id.ID) (workflow.Spec, error)
}

type ConnectorInvoker interface {
	Invoke(ctx context.Context, req connector.InvokeRequest) (connector.InvokeResult, error)
}

type ToolRuntime interface {
	Execute(ctx context.Context, call tool.Call) (tool.Result, error)
}
