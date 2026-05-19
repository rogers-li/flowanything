package httpapi

import (
	"fmt"
	"net/http"
)

func (s *Server) registerRuntimeControlRoutes() {
	s.mux.HandleFunc("GET /v1/runtime/active-bundle", s.handleActiveBundle)
	s.mux.HandleFunc("POST /v1/runtime/reload", s.handleReloadRuntime)
}

func (s *Server) handleActiveBundle(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.runtimeManager.Snapshot())
}

func (s *Server) handleReloadRuntime(w http.ResponseWriter, r *http.Request) {
	if s.platformConfig == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("platform config service is required for runtime reload"))
		return
	}
	var req reloadRuntimeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	bundleID := req.BundleID
	if bundleID == "" {
		bundleID = s.runtimeManager.Snapshot().BundleID
	}
	bundle, err := s.platformConfig.GetReleaseBundle(r.Context(), bundleID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	snapshot, err := s.runtimeManager.Reload(r.Context(), bundle)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

type reloadRuntimeRequest struct {
	BundleID string `json:"bundle_id"`
}
