package httpapi

import (
	"encoding/json"
	"net/http"

	"flow-anything/internal/connector/application"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/kernel/httpserver"
)

func RegisterRoutes(mux *http.ServeMux, app *application.Service) {
	mux.HandleFunc("POST /v1/connector/invoke", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req connector.InvokeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid connector invoke json",
				},
			})
			return
		}

		resp, err := app.Invoke(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})
}
