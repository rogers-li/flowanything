package httpapi

import (
	"encoding/json"
	"net/http"

	"flow-anything/internal/knowledge/application"
	"flow-anything/internal/platform/contracts/knowledge"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func RegisterRoutes(mux *http.ServeMux, app *application.Service) {
	mux.HandleFunc("POST /v1/knowledge/bases", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req knowledge.Base
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid knowledge base json")
			return
		}

		resp, err := app.CreateKnowledgeBase(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("GET /v1/knowledge/bases", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		resp, err := app.ListKnowledgeBases(r.Context(), tenantID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, knowledge.BaseListResponse{Items: resp})
	})

	mux.HandleFunc("GET /v1/knowledge/bases/{kb_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		kbID := id.ID(r.PathValue("kb_id"))
		resp, err := app.GetKnowledgeBase(r.Context(), tenantID, kbID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("PUT /v1/knowledge/bases/{kb_id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		kbID := id.ID(r.PathValue("kb_id"))
		var req knowledge.Base
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid knowledge base json")
			return
		}
		req.TenantID = tenantID
		req.ID = kbID
		resp, err := app.UpdateKnowledgeBase(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/knowledge/bases/{kb_id}/enable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		kbID := id.ID(r.PathValue("kb_id"))
		resp, err := app.SetKnowledgeBaseStatus(r.Context(), tenantID, kbID, knowledge.BaseStatusEnabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/knowledge/bases/{kb_id}/disable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		kbID := id.ID(r.PathValue("kb_id"))
		resp, err := app.SetKnowledgeBaseStatus(r.Context(), tenantID, kbID, knowledge.BaseStatusDisabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("GET /v1/knowledge/bases/{kb_id}/documents", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		kbID := id.ID(r.PathValue("kb_id"))
		resp, err := app.ListDocuments(r.Context(), tenantID, kbID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, knowledge.DocumentListResponse{Items: resp})
	})

	mux.HandleFunc("POST /v1/knowledge/documents", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req knowledge.Document
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid knowledge document json",
				},
			})
			return
		}

		resp, err := app.IndexDocument(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("POST /v1/knowledge/search", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req knowledge.Query
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid knowledge query json",
				},
			})
			return
		}

		resp, err := app.Search(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})
}

func writeInvalidJSON(w http.ResponseWriter, message string) {
	httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
		"error": map[string]string{
			"code":    "invalid_json",
			"message": message,
		},
	})
}
