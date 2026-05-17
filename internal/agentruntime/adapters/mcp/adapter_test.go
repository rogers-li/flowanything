package mcp

import (
	"context"
	"testing"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestExecuteCallsMCPTool(t *testing.T) {
	t.Parallel()

	caller := &fakeCaller{
		result: tool.BackendResult{
			Success: true,
			Data:    map[string]any{"ticket_id": "t_1"},
		},
	}
	adapter := New(caller)

	result, err := adapter.Execute(context.Background(), tool.Spec{
		ID:             id.ID("tool_create_ticket"),
		Implementation: tool.ImplementationMCP,
		Binding: tool.Binding{
			MCPServerID: id.ID("mcp_jira"),
			MCPToolName: "create_ticket",
		},
	}, tool.Call{
		ID:       id.ID("call_1"),
		TenantID: tenant.ID("tenant_1"),
		Args:     map[string]any{"title": "bug"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if caller.req.ServerID != id.ID("mcp_jira") {
		t.Fatalf("expected server binding, got %q", caller.req.ServerID)
	}
	if caller.req.ToolName != "create_ticket" {
		t.Fatalf("expected mcp tool name, got %q", caller.req.ToolName)
	}
}

func TestExecuteRequiresMCPBinding(t *testing.T) {
	t.Parallel()

	_, err := New(&fakeCaller{}).Execute(context.Background(), tool.Spec{
		ID:             id.ID("tool_mcp"),
		Implementation: tool.ImplementationMCP,
	}, tool.Call{ID: id.ID("call_1")})
	if err == nil {
		t.Fatal("expected binding error")
	}
}

type fakeCaller struct {
	req    tool.MCPCallRequest
	result tool.BackendResult
}

func (c *fakeCaller) Call(ctx context.Context, req tool.MCPCallRequest) (tool.BackendResult, error) {
	c.req = req
	return c.result, nil
}
