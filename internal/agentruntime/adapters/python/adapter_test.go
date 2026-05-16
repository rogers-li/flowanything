package python

import (
	"context"
	"testing"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestExecuteRunsPythonPackage(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		result: tool.BackendResult{
			Success: true,
			Data:    map[string]any{"normalized": true},
		},
	}
	adapter := New(runner)

	result, err := adapter.Execute(context.Background(), tool.Spec{
		ID:             id.ID("tool_python"),
		Implementation: tool.ImplementationPython,
		Binding: tool.Binding{
			PythonPackageID: id.ID("pkg_normalize_order"),
		},
	}, tool.Call{
		ID:       id.ID("call_1"),
		TenantID: tenant.ID("tenant_1"),
		Args:     map[string]any{"order_id": "o_1"},
		TraceID:  "trace_1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if runner.req.PackageID != id.ID("pkg_normalize_order") {
		t.Fatalf("expected package binding, got %q", runner.req.PackageID)
	}
	if runner.req.TraceID != "trace_1" {
		t.Fatalf("expected trace id, got %q", runner.req.TraceID)
	}
}

func TestExecuteRequiresPythonPackageBinding(t *testing.T) {
	t.Parallel()

	_, err := New(&fakeRunner{}).Execute(context.Background(), tool.Spec{
		ID:             id.ID("tool_python"),
		Implementation: tool.ImplementationPython,
	}, tool.Call{ID: id.ID("call_1")})
	if err == nil {
		t.Fatal("expected binding error")
	}
}

type fakeRunner struct {
	req    tool.PythonRunRequest
	result tool.BackendResult
}

func (r *fakeRunner) Run(ctx context.Context, req tool.PythonRunRequest) (tool.BackendResult, error) {
	r.req = req
	return r.result, nil
}
