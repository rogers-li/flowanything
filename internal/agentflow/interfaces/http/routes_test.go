package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"flow-anything/internal/agentflow/application"
	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/infrastructure"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestRunAgentFlowRouteReturnsRunAndNodeRuns(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	executor := application.NewExecutor(slog.Default(), store, application.NewNodeRegistry(), nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, executor, nil, store, nil)

	body, err := json.Marshal(runFlowRequest{
		Graph: domain.FlowGraph{
			ID:          "flow_smoke",
			TenantID:    tenant.ID("tenant_1"),
			Name:        "Smoke Flow",
			Status:      domain.FlowStatusEnabled,
			Version:     "v1",
			EntryNodeID: "start",
			Nodes: map[id.ID]domain.Node{
				"start": {ID: "start", Type: domain.NodeTypeStart, Name: "Start"},
			},
		},
		Input: map[string]any{"message": "hello"},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agent-flows/run", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var decoded runFlowResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded.Run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run status = %s, want succeeded", decoded.Run.Status)
	}
	if len(decoded.NodeRuns) != 1 {
		t.Fatalf("node run records = %d, want 1 finished record", len(decoded.NodeRuns))
	}
	if decoded.NodeRuns[0].Status != domain.NodeRunStatusSucceeded {
		t.Fatalf("node status = %s, want succeeded", decoded.NodeRuns[0].Status)
	}
}

func TestRunAgentFlowRouteLoadsGraphByFlowID(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	executor := application.NewExecutor(slog.Default(), store, application.NewNodeRegistry(), nil)
	loader := &fakeAgentFlowLoader{
		spec: agentflow.Spec{
			ID:       id.ID("flow_saved"),
			TenantID: tenant.ID("tenant_1"),
			Name:     "Saved Flow",
			Status:   agentflow.StatusEnabled,
			Graph: domain.FlowGraph{
				ID:          "flow_saved",
				TenantID:    tenant.ID("tenant_1"),
				Name:        "Saved Flow",
				Status:      domain.FlowStatusEnabled,
				Version:     "v1",
				EntryNodeID: "start",
				Nodes: map[id.ID]domain.Node{
					"start": {ID: "start", Type: domain.NodeTypeStart, Name: "Start"},
				},
			},
			Version: "v1",
		},
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, executor, nil, store, loader)

	body, err := json.Marshal(runFlowRequest{
		TenantID: tenant.ID("tenant_1"),
		FlowID:   id.ID("flow_saved"),
		Input:    map[string]any{"message": "hello"},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agent-flows/run", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if loader.tenantID != "tenant_1" || loader.flowID != "flow_saved" {
		t.Fatalf("loader called with tenant=%s flow=%s", loader.tenantID, loader.flowID)
	}

	var decoded runFlowResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded.Run.FlowID != "flow_saved" {
		t.Fatalf("run flow_id = %s, want flow_saved", decoded.Run.FlowID)
	}
}

type fakeAgentFlowLoader struct {
	tenantID tenant.ID
	flowID   id.ID
	spec     agentflow.Spec
	err      error
}

func (l *fakeAgentFlowLoader) LoadAgentFlow(ctx context.Context, tenantID tenant.ID, flowID id.ID) (agentflow.Spec, error) {
	l.tenantID = tenantID
	l.flowID = flowID
	if l.err != nil {
		return agentflow.Spec{}, l.err
	}
	return l.spec, nil
}
