package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	connectoradapter "flow-anything/internal/agentruntime/adapters/connector"
	"flow-anything/internal/agentruntime/infrastructure"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestExecuteConnectorToolUsesCatalogAndAdapter(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	toolID := id.ID("tool_query_order")
	operationID := id.ID("connop_query_order")

	catalog := infrastructure.NewMemoryToolCatalog([]tool.Spec{
		{
			ID:             toolID,
			TenantID:       tenantID,
			Name:           "query_order",
			Implementation: tool.ImplementationConnector,
			Binding: tool.Binding{
				ConnectorOperationID: operationID,
			},
		},
	})
	invoker := infrastructure.NewMemoryConnectorInvoker()
	recorder := infrastructure.NewMemoryExecutionRecorder()
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		catalog,
		recorder,
		connectoradapter.New(invoker),
	)

	result, err := service.ExecuteTool(context.Background(), tool.Call{
		TenantID: tenantID,
		ToolID:   toolID,
		Args: map[string]any{
			"order_id": "o_123",
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTool() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected successful tool result")
	}
	if result.ToolID != toolID {
		t.Fatalf("expected tool id %q, got %q", toolID, result.ToolID)
	}
	record, ok := recorder.Get(result.CallID.String())
	if !ok {
		t.Fatal("expected execution record")
	}
	if record.Status != tool.ExecutionStatusSucceeded {
		t.Fatalf("expected succeeded audit status, got %q", record.Status)
	}
	if record.ToolName != "query_order" {
		t.Fatalf("expected tool name query_order, got %q", record.ToolName)
	}
	if record.ArgsSummary["order_id"] == nil {
		t.Fatal("expected args summary for order_id")
	}
	fetched, err := service.GetExecution(context.Background(), tenantID, result.CallID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if fetched.CallID != result.CallID {
		t.Fatalf("expected fetched call id %q, got %q", result.CallID, fetched.CallID)
	}
}

func TestExecuteToolValidatesInputSchema(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	toolID := id.ID("tool_query_order")
	catalog := infrastructure.NewMemoryToolCatalog([]tool.Spec{
		{
			ID:             toolID,
			TenantID:       tenantID,
			Name:           "query_order",
			Implementation: tool.ImplementationWorkflow,
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"order_id"},
				"properties": map[string]any{
					"order_id": map[string]any{"type": "string"},
				},
			},
		},
	})
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		catalog,
		infrastructure.NewMemoryExecutionRecorder(),
		&fakeAdapter{kind: tool.ImplementationWorkflow},
	)

	result, err := service.ExecuteTool(context.Background(), tool.Call{
		TenantID: tenantID,
		ToolID:   toolID,
		Args:     map[string]any{},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if result.ErrorCode != "invalid_arguments" {
		t.Fatalf("expected invalid_arguments, got %q", result.ErrorCode)
	}
	record, ok := service.recorder.(*infrastructure.MemoryExecutionRecorder).Get(result.CallID.String())
	if !ok {
		t.Fatal("expected failed execution record")
	}
	if record.Status != tool.ExecutionStatusFailed || record.ErrorCode != "invalid_arguments" {
		t.Fatalf("unexpected audit record %#v", record)
	}
}

func TestExecuteToolAllowsHighRiskToolUntilConfirmationFlowIsSupported(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	toolID := id.ID("tool_refund_order")
	catalog := infrastructure.NewMemoryToolCatalog([]tool.Spec{
		{
			ID:             toolID,
			TenantID:       tenantID,
			Name:           "refund_order",
			Implementation: tool.ImplementationWorkflow,
			RiskLevel:      tool.RiskHigh,
		},
	})
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		catalog,
		infrastructure.NewMemoryExecutionRecorder(),
		&fakeAdapter{kind: tool.ImplementationWorkflow},
	)

	result, err := service.ExecuteTool(context.Background(), tool.Call{
		TenantID: tenantID,
		ToolID:   toolID,
	})
	if err != nil {
		t.Fatalf("ExecuteTool() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("expected successful execution while confirmation is disabled, got %#v", result)
	}
}

func TestExecuteToolHonorsTimeout(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	toolID := id.ID("tool_slow")
	catalog := infrastructure.NewMemoryToolCatalog([]tool.Spec{
		{
			ID:             toolID,
			TenantID:       tenantID,
			Name:           "slow_tool",
			Implementation: tool.ImplementationWorkflow,
			TimeoutMillis:  5,
		},
	})
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		catalog,
		infrastructure.NewMemoryExecutionRecorder(),
		&fakeAdapter{kind: tool.ImplementationWorkflow, waitForCancel: true},
	)

	result, err := service.ExecuteTool(context.Background(), tool.Call{
		TenantID: tenantID,
		ToolID:   toolID,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if result.ErrorCode != "timeout" {
		t.Fatalf("expected timeout, got %q", result.ErrorCode)
	}
}

type fakeAdapter struct {
	kind          tool.ImplementationType
	waitForCancel bool
}

func (a *fakeAdapter) Supports(kind tool.ImplementationType) bool {
	return kind == a.kind
}

func (a *fakeAdapter) Execute(ctx context.Context, spec tool.Spec, call tool.Call) (tool.Result, error) {
	if a.waitForCancel {
		<-ctx.Done()
		return tool.Result{}, ctx.Err()
	}

	return tool.Result{
		CallID:  call.ID,
		ToolID:  spec.ID,
		Success: true,
	}, nil
}
