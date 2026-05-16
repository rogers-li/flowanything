package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHTTPToolRuntimeExecuteTool(t *testing.T) {
	t.Parallel()

	toolID := id.ID("tool_1")
	client := httpclient.NewWithHTTPClient("http://runtime.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/tools/execute" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}

			var req tool.Call
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if req.ToolID != toolID {
				t.Fatalf("expected tool id %q, got %q", toolID, req.ToolID)
			}

			var body strings.Builder
			_ = json.NewEncoder(&body).Encode(tool.Result{
				CallID:     req.ID,
				ToolID:     req.ToolID,
				Success:    true,
				Data:       map[string]any{"ok": true},
				FinishedAt: time.Now().UTC(),
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body.String())),
			}, nil
		}),
	})

	runtime := &HTTPToolRuntime{client: client}
	result, err := runtime.ExecuteTool(context.Background(), tool.Call{
		TenantID: tenant.ID("tenant_1"),
		ToolID:   toolID,
	})
	if err != nil {
		t.Fatalf("ExecuteTool() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}
