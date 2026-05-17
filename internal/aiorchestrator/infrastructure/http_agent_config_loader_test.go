package infrastructure

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHTTPAgentConfigLoaderKeepsConfiguredToolOrder(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Query().Get("tenant_id") != "tenant_1" {
				t.Fatalf("expected tenant query, got %q", r.URL.RawQuery)
			}
			body := ""
			statusCode := http.StatusOK

			switch r.URL.Path {
			case "/v1/agents/agent_1":
				body = `{"id":"agent_1","tenant_id":"tenant_1","name":"Order Agent","tool_ids":["tool_direct","tool_a"],"skill_ids":["skill_a","skill_b"]}`
			case "/v1/skills/skill_a":
				body = `{"id":"skill_a","tenant_id":"tenant_1","name":"skill a","tool_ids":["tool_a","tool_b"]}`
			case "/v1/skills/skill_b":
				body = `{"id":"skill_b","tenant_id":"tenant_1","name":"skill b","tool_ids":["tool_b","tool_c"]}`
			case "/v1/tools/tool_a":
				body = `{"id":"tool_a","tenant_id":"tenant_1","name":"tool_a","implementation":"connector","binding":{"connector_operation_id":"connop_a"}}`
			case "/v1/tools/tool_b":
				body = `{"id":"tool_b","tenant_id":"tenant_1","name":"tool_b","implementation":"connector","binding":{"connector_operation_id":"connop_b"}}`
			case "/v1/tools/tool_c":
				body = `{"id":"tool_c","tenant_id":"tenant_1","name":"tool_c","implementation":"connector","binding":{"connector_operation_id":"connop_c"}}`
			case "/v1/tools/tool_direct":
				body = `{"id":"tool_direct","tenant_id":"tenant_1","name":"tool_direct","implementation":"connector","binding":{"connector_operation_id":"connop_direct"}}`
			default:
				body = `{"error":{"code":"not_found","message":"not found"}}`
				statusCode = http.StatusNotFound
			}

			return &http.Response{
				StatusCode: statusCode,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}

	loader := NewHTTPAgentConfigLoaderWithClient("https://platform.test", client)
	config, err := loader.LoadAgentConfig(context.Background(), tenant.ID("tenant_1"), id.ID("agent_1"))
	if err != nil {
		t.Fatalf("LoadAgentConfig() error = %v", err)
	}

	got := make([]id.ID, 0, len(config.Tools))
	for _, spec := range config.Tools {
		got = append(got, spec.ID)
	}
	want := []id.ID{"tool_direct", "tool_a", "tool_b", "tool_c"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("expected tool order %v, got %v", want, got)
	}
}
