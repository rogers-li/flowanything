package workflow

import (
	"context"
	"testing"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestExecuteRunsWorkflow(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		result: tool.BackendResult{
			Success: true,
			Data:    map[string]any{"workflow_status": "completed"},
		},
	}
	adapter := New(runner)

	result, err := adapter.Execute(context.Background(), tool.Spec{
		ID:             id.ID("tool_refund"),
		Implementation: tool.ImplementationWorkflow,
		Binding: tool.Binding{
			WorkflowID: id.ID("workflow_refund_order"),
		},
	}, tool.Call{
		ID:       id.ID("call_1"),
		TenantID: tenant.ID("tenant_1"),
		Args:     map[string]any{"order_id": "o_1"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if runner.req.WorkflowID != id.ID("workflow_refund_order") {
		t.Fatalf("expected workflow binding, got %q", runner.req.WorkflowID)
	}
}

func TestExecuteRequiresWorkflowBinding(t *testing.T) {
	t.Parallel()

	_, err := New(&fakeRunner{}).Execute(context.Background(), tool.Spec{
		ID:             id.ID("tool_workflow"),
		Implementation: tool.ImplementationWorkflow,
	}, tool.Call{ID: id.ID("call_1")})
	if err == nil {
		t.Fatal("expected binding error")
	}
}

type fakeRunner struct {
	req    tool.WorkflowRunRequest
	result tool.BackendResult
}

func (r *fakeRunner) Run(ctx context.Context, req tool.WorkflowRunRequest) (tool.BackendResult, error) {
	r.req = req
	return r.result, nil
}
