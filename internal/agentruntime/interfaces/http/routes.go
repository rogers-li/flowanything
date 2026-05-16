package httpapi

import (
	"encoding/json"
	"net/http"

	"flow-anything/internal/agentruntime/application"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func RegisterRoutes(mux *http.ServeMux, app *application.Service) {
	mux.HandleFunc("GET /v1/tool-executions/{call_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		callID := id.ID(r.PathValue("call_id"))

		resp, err := app.GetExecution(r.Context(), tenantID, callID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/tools/execute", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req tool.Call
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid tool call json",
				},
			})
			return
		}

		resp, err := app.ExecuteTool(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})
}
