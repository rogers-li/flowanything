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

func TestHTTPToolCatalogGetTool(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	toolID := id.ID("tool_1")

	client := httpclient.NewWithHTTPClient("http://platform.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/tools/tool_1" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}
			if r.URL.Query().Get("tenant_id") != tenantID.String() {
				t.Fatalf("unexpected tenant_id %q", r.URL.Query().Get("tenant_id"))
			}

			var body strings.Builder
			_ = json.NewEncoder(&body).Encode(tool.Spec{
				ID:             toolID,
				TenantID:       tenantID,
				Name:           "query_order",
				Implementation: tool.ImplementationConnector,
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body.String())),
			}, nil
		}),
	})

	catalog := &HTTPToolCatalog{client: client}
	spec, err := catalog.GetTool(context.Background(), tool.Call{
		TenantID: tenantID,
		ToolID:   toolID,
	})
	if err != nil {
		t.Fatalf("GetTool() error = %v", err)
	}
	if spec.ID != toolID {
		t.Fatalf("expected tool id %q, got %q", toolID, spec.ID)
	}
}
