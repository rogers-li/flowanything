package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHTTPConnectorInvokerInvoke(t *testing.T) {
	t.Parallel()

	requestID := id.ID("connreq_1")
	operationID := id.ID("connop_1")

	client := httpclient.NewWithHTTPClient("http://connector.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/connector/invoke" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}

			var req connector.InvokeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if req.OperationID != operationID {
				t.Fatalf("expected operation id %q, got %q", operationID, req.OperationID)
			}

			var body strings.Builder
			_ = json.NewEncoder(&body).Encode(connector.InvokeResult{
				RequestID:  req.ID,
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

	invoker := &HTTPConnectorInvoker{client: client}
	result, err := invoker.Invoke(context.Background(), connector.InvokeRequest{
		ID:          requestID,
		TenantID:    tenant.ID("tenant_1"),
		OperationID: operationID,
		Args:        map[string]any{"order_id": "o_1"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}
