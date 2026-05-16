package workflowruntime

import (
	"encoding/json"
	"net/http"
	"strconv"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func RegisterRoutes(mux *http.ServeMux, app *Service) {
	mux.HandleFunc("POST /v1/workflows/run", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req workflow.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid workflow run json",
				},
			})
			return
		}

		resp, err := app.RunWorkflow(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("GET /v1/workflows/runs", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		workflowID := id.ID(r.URL.Query().Get("workflow_id"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		resp, err := app.ListRuns(r.Context(), tenantID, workflowID, limit)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, workflow.RunListResponse{Items: resp})
	})

	mux.HandleFunc("GET /v1/workflows/runs/{run_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		runID := id.ID(r.PathValue("run_id"))
		resp, err := app.GetRunResponse(r.Context(), tenantID, runID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/workflows/runs/{run_id}/replay", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		runID := id.ID(r.PathValue("run_id"))
		var req workflow.ReplayRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid workflow replay json",
				},
			})
			return
		}
		resp, err := app.ReplayRun(r.Context(), tenantID, runID, req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/tools/workflows/run", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req tool.WorkflowRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid workflow tool run json",
				},
			})
			return
		}

		resp, err := app.RunToolWorkflow(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})
}
