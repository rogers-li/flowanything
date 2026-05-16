package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHTTPPythonRunnerRun(t *testing.T) {
	t.Parallel()

	client := httpclient.NewWithHTTPClient("http://code-adapter.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/code-adapter/runs" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}
			var req tool.PythonRunRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if req.PackageID != id.ID("pkg_1") {
				t.Fatalf("expected package id, got %q", req.PackageID)
			}
			return jsonResponse(tool.BackendResult{
				RequestID: req.ID,
				Success:   true,
				Data:      map[string]any{"ok": true},
			}), nil
		}),
	})

	runner := &HTTPPythonRunner{client: client}
	result, err := runner.Run(context.Background(), tool.PythonRunRequest{
		ID:        id.ID("pyrun_1"),
		TenantID:  tenant.ID("tenant_1"),
		ToolID:    id.ID("tool_1"),
		PackageID: id.ID("pkg_1"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestHTTPMCPCallerCall(t *testing.T) {
	t.Parallel()

	client := httpclient.NewWithHTTPClient("http://mcp.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/mcp/tools/call" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}
			var req tool.MCPCallRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if req.ToolName != "create_ticket" {
				t.Fatalf("expected tool name, got %q", req.ToolName)
			}
			return jsonResponse(tool.BackendResult{
				RequestID: req.ID,
				Success:   true,
				Data:      map[string]any{"ticket_id": "t_1"},
			}), nil
		}),
	})

	caller := &HTTPMCPCaller{client: client}
	result, err := caller.Call(context.Background(), tool.MCPCallRequest{
		ID:       id.ID("mcpcall_1"),
		TenantID: tenant.ID("tenant_1"),
		ToolID:   id.ID("tool_1"),
		ServerID: id.ID("mcp_jira"),
		ToolName: "create_ticket",
	})
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestHTTPWorkflowRunnerRun(t *testing.T) {
	t.Parallel()

	client := httpclient.NewWithHTTPClient("http://workflow.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/tools/workflows/run" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}
			var req tool.WorkflowRunRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if req.WorkflowID != id.ID("workflow_1") {
				t.Fatalf("expected workflow id, got %q", req.WorkflowID)
			}
			return jsonResponse(tool.BackendResult{
				RequestID: req.ID,
				Success:   true,
				Data:      map[string]any{"status": "completed"},
			}), nil
		}),
	})

	runner := &HTTPWorkflowRunner{client: client}
	result, err := runner.Run(context.Background(), tool.WorkflowRunRequest{
		ID:         id.ID("wfrun_1"),
		TenantID:   tenant.ID("tenant_1"),
		ToolID:     id.ID("tool_1"),
		WorkflowID: id.ID("workflow_1"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func jsonResponse(body any) *http.Response {
	var payload strings.Builder
	_ = json.NewEncoder(&payload).Encode(body)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(payload.String())),
	}
}
