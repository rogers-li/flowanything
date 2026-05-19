package httpapi

import (
	"fmt"
	"net/http"
	"time"

	coreconfig "flow-anything/core/config"
	configadapter "flow-anything/internal_new/adapters/config"
	"flow-anything/internal_new/platformconfig"
)

func (s *Server) registerPlatformConfigRoutes() {
	s.mux.HandleFunc("GET /v1/bundles", s.handleListBundles)
	s.mux.HandleFunc("POST /v1/bundles", s.handleSaveBundle)
	s.mux.HandleFunc("POST /v1/bundles/validate", s.handleValidateBundle)
	s.mux.HandleFunc("GET /v1/previews", s.handleListPreviewBundles)
	s.mux.HandleFunc("GET /v1/releases", s.handleListReleaseBundles)
	s.mux.HandleFunc("GET /v1/bundles/{bundle_id}", s.handleGetBundle)
	s.mux.HandleFunc("PUT /v1/bundles/{bundle_id}", s.handleUpdateBundle)
	s.mux.HandleFunc("DELETE /v1/bundles/{bundle_id}", s.handleDeleteBundle)
	s.mux.HandleFunc("GET /v1/bundles/{bundle_id}/inspect", s.handleInspectBundle)
	s.mux.HandleFunc("POST /v1/bundles/{bundle_id}/validate", s.handleValidateStoredBundle)
	s.mux.HandleFunc("POST /v1/bundles/{bundle_id}/preview", s.handleBuildPreviewBundle)
	s.mux.HandleFunc("POST /v1/bundles/{bundle_id}/publish", s.handlePublishBundle)
	s.mux.HandleFunc("POST /v1/bundles/{bundle_id}/publish-and-reload", s.handlePublishAndReloadBundle)
	s.mux.HandleFunc("GET /v1/bundles/{bundle_id}/resources", s.handleListResources)
	s.mux.HandleFunc("GET /v1/bundles/{bundle_id}/resources/{resource_kind}", s.handleListResourcesByKind)
	s.mux.HandleFunc("GET /v1/bundles/{bundle_id}/resources/{resource_kind}/{resource_id}", s.handleGetResource)
	s.mux.HandleFunc("PUT /v1/bundles/{bundle_id}/resources/{resource_kind}/{resource_id}", s.handleUpsertResource)
	s.mux.HandleFunc("DELETE /v1/bundles/{bundle_id}/resources/{resource_kind}/{resource_id}", s.handleDeleteResource)
	s.mux.HandleFunc("GET /v1/bundles/{bundle_id}/connectors/{connector_id}/operations", s.handleListConnectorOperations)
	s.mux.HandleFunc("GET /v1/bundles/{bundle_id}/connectors/{connector_id}/operations/{operation_id}", s.handleGetConnectorOperation)
	s.mux.HandleFunc("PUT /v1/bundles/{bundle_id}/connectors/{connector_id}/operations/{operation_id}", s.handleUpsertConnectorOperation)
	s.mux.HandleFunc("DELETE /v1/bundles/{bundle_id}/connectors/{connector_id}/operations/{operation_id}", s.handleDeleteConnectorOperation)
}

func (s *Server) handleListBundles(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.platformConfig.ListBundles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]bundleSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, toBundleSummaryResponse(summary))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleListPreviewBundles(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.platformConfig.ListPreviewBundles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]bundleSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, toBundleSummaryResponse(summary))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleListReleaseBundles(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.platformConfig.ListReleaseBundles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]bundleSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, toBundleSummaryResponse(summary))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleGetBundle(w http.ResponseWriter, r *http.Request) {
	bundle, err := s.platformConfig.GetBundle(r.Context(), r.PathValue("bundle_id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, bundleResponse{Bundle: bundle})
}

func (s *Server) handleSaveBundle(w http.ResponseWriter, r *http.Request) {
	var bundle coreconfig.BundleSpec
	if !decodeJSON(w, r, &bundle) {
		return
	}
	saved, err := s.platformConfig.SaveBundle(r.Context(), bundle)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, bundleResponse{Bundle: saved})
}

func (s *Server) handleUpdateBundle(w http.ResponseWriter, r *http.Request) {
	var bundle coreconfig.BundleSpec
	if !decodeJSON(w, r, &bundle) {
		return
	}
	bundleID := r.PathValue("bundle_id")
	if bundle.ID == "" {
		bundle.ID = bundleID
	}
	if bundle.ID != bundleID {
		writeError(w, http.StatusBadRequest, errPathIDMismatch("bundle", bundleID, bundle.ID))
		return
	}
	saved, err := s.platformConfig.SaveBundle(r.Context(), bundle)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, bundleResponse{Bundle: saved})
}

func (s *Server) handleDeleteBundle(w http.ResponseWriter, r *http.Request) {
	if err := s.platformConfig.DeleteBundle(r.Context(), r.PathValue("bundle_id")); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handleInspectBundle(w http.ResponseWriter, r *http.Request) {
	state, err := s.platformConfig.InspectBundle(
		r.Context(),
		r.PathValue("bundle_id"),
		coreconfig.BundleLifecycle(r.URL.Query().Get("lifecycle")),
	)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleValidateBundle(w http.ResponseWriter, r *http.Request) {
	var bundle coreconfig.BundleSpec
	if !decodeJSON(w, r, &bundle) {
		return
	}
	writeJSON(w, http.StatusOK, s.platformConfig.ValidateBundle(bundle))
}

func (s *Server) handleValidateStoredBundle(w http.ResponseWriter, r *http.Request) {
	result, err := s.platformConfig.ValidateStoredBundle(r.Context(), r.PathValue("bundle_id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePublishBundle(w http.ResponseWriter, r *http.Request) {
	result, err := s.platformConfig.PublishBundle(r.Context(), r.PathValue("bundle_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePublishAndReloadBundle(w http.ResponseWriter, r *http.Request) {
	if s.runtimeManager == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("runtime manager is required for publish-and-reload"))
		return
	}
	result, err := s.platformConfig.PublishBundle(r.Context(), r.PathValue("bundle_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	releaseBundle, err := s.platformConfig.GetReleaseBundle(r.Context(), result.BundleID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	snapshot, err := s.runtimeManager.Reload(r.Context(), releaseBundle)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"publish": result,
		"runtime": snapshot,
	})
}

func (s *Server) handleBuildPreviewBundle(w http.ResponseWriter, r *http.Request) {
	var req buildPreviewBundleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	preview, info, err := s.platformConfig.BuildPreviewBundle(r.Context(), r.PathValue("bundle_id"), req.Entrypoint)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, previewBundleResponse{Bundle: preview, Preview: info})
}

func (s *Server) handleListResources(w http.ResponseWriter, r *http.Request) {
	items, err := s.platformConfig.ListResources(r.Context(), r.PathValue("bundle_id"), platformconfig.ResourceListFilter{
		Kind:  coreconfig.ResourceKind(r.URL.Query().Get("kind")),
		Query: r.URL.Query().Get("q"),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, resourceListResponse{Items: items})
}

func (s *Server) handleListResourcesByKind(w http.ResponseWriter, r *http.Request) {
	items, err := s.platformConfig.ListResources(r.Context(), r.PathValue("bundle_id"), platformconfig.ResourceListFilter{
		Kind:  coreconfig.ResourceKind(r.PathValue("resource_kind")),
		Query: r.URL.Query().Get("q"),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, resourceListResponse{Items: items})
}

func (s *Server) handleGetResource(w http.ResponseWriter, r *http.Request) {
	resource, err := s.platformConfig.GetResource(
		r.Context(),
		r.PathValue("bundle_id"),
		coreconfig.ResourceKind(r.PathValue("resource_kind")),
		r.PathValue("resource_id"),
	)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, resource)
}

func (s *Server) handleUpsertResource(w http.ResponseWriter, r *http.Request) {
	resource, ok := s.decodeResource(w, r, coreconfig.ResourceKind(r.PathValue("resource_kind")), r.PathValue("resource_id"))
	if !ok {
		return
	}
	bundle, err := s.platformConfig.UpsertResource(r.Context(), r.PathValue("bundle_id"), resource)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, bundleResponse{Bundle: bundle})
}

func (s *Server) handleDeleteResource(w http.ResponseWriter, r *http.Request) {
	bundle, err := s.platformConfig.DeleteResource(
		r.Context(),
		r.PathValue("bundle_id"),
		coreconfig.ResourceKind(r.PathValue("resource_kind")),
		r.PathValue("resource_id"),
	)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, bundleResponse{Bundle: bundle})
}

func (s *Server) handleListConnectorOperations(w http.ResponseWriter, r *http.Request) {
	operations, err := s.platformConfig.ListConnectorOperations(
		r.Context(),
		r.PathValue("bundle_id"),
		r.PathValue("connector_id"),
		r.URL.Query().Get("q"),
	)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, connectorOperationListResponse{Items: operations})
}

func (s *Server) handleGetConnectorOperation(w http.ResponseWriter, r *http.Request) {
	operation, err := s.platformConfig.GetConnectorOperation(
		r.Context(),
		r.PathValue("bundle_id"),
		r.PathValue("connector_id"),
		r.PathValue("operation_id"),
	)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, connectorOperationResponse{Operation: operation})
}

func (s *Server) handleUpsertConnectorOperation(w http.ResponseWriter, r *http.Request) {
	var operation coreconfig.ConnectorOperationConfig
	if !decodeJSON(w, r, &operation) {
		return
	}
	operationID := r.PathValue("operation_id")
	if operation.ID == "" {
		operation.ID = operationID
	}
	if operation.ID != operationID {
		writeError(w, http.StatusBadRequest, errPathIDMismatch("connector operation", operationID, operation.ID))
		return
	}
	bundle, err := s.platformConfig.UpsertConnectorOperation(
		r.Context(),
		r.PathValue("bundle_id"),
		r.PathValue("connector_id"),
		operation,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, bundleResponse{Bundle: bundle})
}

func (s *Server) handleDeleteConnectorOperation(w http.ResponseWriter, r *http.Request) {
	bundle, err := s.platformConfig.DeleteConnectorOperation(
		r.Context(),
		r.PathValue("bundle_id"),
		r.PathValue("connector_id"),
		r.PathValue("operation_id"),
	)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, bundleResponse{Bundle: bundle})
}

type bundleResponse struct {
	Bundle coreconfig.BundleSpec `json:"bundle"`
}

type buildPreviewBundleRequest struct {
	Entrypoint coreconfig.BundleEntrypoint `json:"entrypoint"`
}

type previewBundleResponse struct {
	Bundle  coreconfig.BundleSpec             `json:"bundle"`
	Preview platformconfig.BundleSnapshotInfo `json:"preview"`
}

type resourceListResponse struct {
	Items []platformconfig.ResourceDocument `json:"items"`
}

type connectorOperationListResponse struct {
	Items []coreconfig.ConnectorOperationConfig `json:"items"`
}

type connectorOperationResponse struct {
	Operation coreconfig.ConnectorOperationConfig `json:"operation"`
}

type bundleSummaryResponse struct {
	ID             string                     `json:"id"`
	Name           string                     `json:"name"`
	Version        string                     `json:"version"`
	Lifecycle      coreconfig.BundleLifecycle `json:"lifecycle,omitempty"`
	SourceBundleID string                     `json:"source_bundle_id,omitempty"`
	ContentHash    string                     `json:"content_hash,omitempty"`
	UpdatedAt      time.Time                  `json:"updated_at,omitempty"`
}

func toBundleSummaryResponse(summary configadapter.BundleSummary) bundleSummaryResponse {
	return bundleSummaryResponse{
		ID:             summary.ID,
		Name:           summary.Name,
		Version:        summary.Version,
		Lifecycle:      summary.Lifecycle,
		SourceBundleID: summary.SourceBundleID,
		ContentHash:    summary.ContentHash,
		UpdatedAt:      summary.UpdatedAt,
	}
}

func errPathIDMismatch(kind string, pathID string, bodyID string) error {
	return fmt.Errorf("%s id mismatch: path has %q but body has %q", kind, pathID, bodyID)
}

func (s *Server) decodeResource(w http.ResponseWriter, r *http.Request, kind coreconfig.ResourceKind, resourceID string) (platformconfig.ResourceDocument, bool) {
	resource := platformconfig.ResourceDocument{Kind: kind, ID: resourceID}
	switch kind {
	case coreconfig.ResourceAgent:
		var value coreconfig.AgentConfig
		if !decodeJSON(w, r, &value) {
			return resource, false
		}
		resource.Resource = value
	case coreconfig.ResourceSkill:
		var value coreconfig.SkillConfig
		if !decodeJSON(w, r, &value) {
			return resource, false
		}
		resource.Resource = value
	case coreconfig.ResourceTool:
		var value coreconfig.ToolConfig
		if !decodeJSON(w, r, &value) {
			return resource, false
		}
		resource.Resource = value
	case coreconfig.ResourceWorkflow:
		var value coreconfig.WorkflowConfig
		if !decodeJSON(w, r, &value) {
			return resource, false
		}
		resource.Resource = value
	case coreconfig.ResourceConnector:
		var value coreconfig.ConnectorConfig
		if !decodeJSON(w, r, &value) {
			return resource, false
		}
		resource.Resource = value
	case coreconfig.ResourceModel:
		var value coreconfig.ModelConfig
		if !decodeJSON(w, r, &value) {
			return resource, false
		}
		resource.Resource = value
	case coreconfig.ResourceKnowledge:
		var value coreconfig.KnowledgeConfig
		if !decodeJSON(w, r, &value) {
			return resource, false
		}
		resource.Resource = value
	case coreconfig.ResourcePolicy:
		var value coreconfig.PolicyConfig
		if !decodeJSON(w, r, &value) {
			return resource, false
		}
		resource.Resource = value
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported resource kind %q", kind))
		return resource, false
	}
	return resource, true
}
