package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"flow-anything/internal/agentflow/application"
	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/platform/contracts/agentflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type runFlowRequest struct {
	TenantID tenant.ID        `json:"tenant_id,omitempty"`
	FlowID   id.ID            `json:"flow_id,omitempty"`
	Flow     agentflow.Spec   `json:"flow,omitempty"`
	Graph    domain.FlowGraph `json:"graph,omitempty"`
	Input    map[string]any   `json:"input,omitempty"`
}

type runFlowResponse struct {
	Run      domain.FlowRun   `json:"run"`
	NodeRuns []domain.NodeRun `json:"node_runs,omitempty"`
	Error    string           `json:"error,omitempty"`
}

func RegisterRoutes(mux *http.ServeMux, executor *application.Executor, supervisorRunner *application.SupervisorRunner, store ports.RunStore, loader ports.AgentFlowConfigLoader) {
	mux.HandleFunc("POST /v1/agent-flows/run", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req runFlowRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid agent flow run json",
				},
			})
			return
		}
		if req.Input == nil {
			req.Input = map[string]any{}
		}
		req.Input = normalizeAgentFlowInput(req.Input)

		spec, err := flowForRunRequest(r, req, loader)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		run, err := executeFlow(r, executor, supervisorRunner, spec, req.Input)
		nodeRuns := listNodeRuns(r, store, run)
		if err != nil {
			// A graph can fail because an inner agent/tool node failed. Returning
			// the failed run keeps the debug surface useful for callers.
			if !run.ID.Empty() {
				httpserver.WriteJSON(w, http.StatusOK, runFlowResponse{
					Run:      run,
					NodeRuns: nodeRuns,
					Error:    err.Error(),
				})
				return
			}
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, runFlowResponse{
			Run:      run,
			NodeRuns: nodeRuns,
		})
	})
}

func normalizeAgentFlowInput(input map[string]any) map[string]any {
	for _, key := range []string{"user_request", "task", "message", "text", "query"} {
		if text := strings.TrimSpace(fmt.Sprint(input[key])); text != "" && text != "<nil>" {
			return map[string]any{"user_request": text}
		}
	}
	payload, err := json.Marshal(input)
	if err != nil || string(payload) == "{}" {
		return map[string]any{"user_request": ""}
	}
	return map[string]any{"user_request": string(payload)}
}

func flowForRunRequest(r *http.Request, req runFlowRequest, loader ports.AgentFlowConfigLoader) (agentflow.Spec, error) {
	if !req.FlowID.Empty() {
		if loader == nil {
			return agentflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "agent flow config loader is not configured")
		}
		if req.TenantID.Empty() {
			return agentflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required when flow_id is provided")
		}
		return loader.LoadAgentFlow(r.Context(), req.TenantID, req.FlowID)
	}
	if !req.Flow.ID.Empty() || req.Flow.OrchestrationMode != "" {
		if req.Flow.TenantID.Empty() {
			req.Flow.TenantID = req.TenantID
		}
		return req.Flow, nil
	}
	return agentflow.Spec{
		ID:                req.Graph.ID,
		TenantID:          req.Graph.TenantID,
		Name:              req.Graph.Name,
		Description:       req.Graph.Description,
		Status:            req.Graph.Status,
		OrchestrationMode: agentflow.OrchestrationModeWorkflow,
		Graph:             req.Graph,
		Version:           req.Graph.Version,
	}, nil
}

func executeFlow(r *http.Request, executor *application.Executor, supervisorRunner *application.SupervisorRunner, spec agentflow.Spec, input map[string]any) (domain.FlowRun, error) {
	switch spec.OrchestrationMode {
	case "", agentflow.OrchestrationModeWorkflow:
		if executor == nil {
			return domain.FlowRun{}, apperrors.New(apperrors.CodeUnavailable, "agent flow executor is not configured")
		}
		return executor.Execute(r.Context(), spec.RuntimeGraph(), input)
	case agentflow.OrchestrationModeSupervisor:
		if supervisorRunner == nil {
			return domain.FlowRun{}, apperrors.New(apperrors.CodeUnavailable, "supervisor runner is not configured")
		}
		return supervisorRunner.Execute(r.Context(), spec, input)
	default:
		return domain.FlowRun{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported agent flow orchestration mode")
	}
}

func listNodeRuns(r *http.Request, store ports.RunStore, run domain.FlowRun) []domain.NodeRun {
	if store == nil || run.ID.Empty() {
		return nil
	}
	nodeRuns, err := store.ListNodeRuns(r.Context(), run.TenantID, run.ID)
	if err != nil {
		return nil
	}
	return nodeRuns
}
