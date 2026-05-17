package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	flowdomain "flow-anything/internal/agentflow/domain"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHTTPAgentFlowConfigLoaderLoadsEnabledFlow(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/agent-flows/flow_support" {
			t.Fatalf("path = %s, want /v1/agent-flows/flow_support", r.URL.Path)
		}
		if r.URL.Query().Get("tenant_id") != "tenant_1" {
			t.Fatalf("tenant query = %s, want tenant_1", r.URL.Query().Get("tenant_id"))
		}

		body, err := json.Marshal(agentflow.Spec{
			ID:       id.ID("flow_support"),
			TenantID: tenant.ID("tenant_1"),
			Name:     "Support Flow",
			Status:   agentflow.StatusEnabled,
			Graph: flowdomain.FlowGraph{
				ID:          "flow_support",
				TenantID:    "tenant_1",
				Name:        "Support Flow",
				Status:      flowdomain.FlowStatusEnabled,
				EntryNodeID: "start",
				Nodes: map[id.ID]flowdomain.Node{
					"start": {ID: "start", Type: flowdomain.NodeTypeStart, Name: "Start"},
				},
			},
			Version: "v1",
		})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}

	loader := NewHTTPAgentFlowConfigLoaderWithClient("http://platform.test", client)
	spec, err := loader.LoadAgentFlow(context.Background(), tenant.ID("tenant_1"), id.ID("flow_support"))
	if err != nil {
		t.Fatalf("LoadAgentFlow() error = %v", err)
	}
	if spec.Graph.EntryNodeID != "start" {
		t.Fatalf("entry node = %s, want start", spec.Graph.EntryNodeID)
	}
}

func TestHTTPAgentFlowConfigLoaderRejectsDisabledFlow(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := json.Marshal(agentflow.Spec{
			ID:       id.ID("flow_disabled"),
			TenantID: tenant.ID("tenant_1"),
			Name:     "Disabled Flow",
			Status:   agentflow.StatusDisabled,
		})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}

	loader := NewHTTPAgentFlowConfigLoaderWithClient("http://platform.test", client)
	if _, err := loader.LoadAgentFlow(context.Background(), tenant.ID("tenant_1"), id.ID("flow_disabled")); err == nil {
		t.Fatal("LoadAgentFlow() error = nil, want disabled flow error")
	}
}
