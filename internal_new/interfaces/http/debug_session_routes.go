package httpapi

import (
	"fmt"
	"net/http"
	"time"

	coreconfig "flow-anything/core/config"
	"flow-anything/internal_new/app"
	"flow-anything/internal_new/platformconfig"
)

func (s *Server) registerDebugSessionRoutes() {
	s.mux.HandleFunc("GET /v1/debug-sessions", s.handleListDebugSessions)
	s.mux.HandleFunc("POST /v1/debug-sessions", s.handleCreateDebugSession)
	s.mux.HandleFunc("GET /v1/debug-sessions/{session_id}", s.handleGetDebugSession)
	s.mux.HandleFunc("DELETE /v1/debug-sessions/{session_id}", s.handleDeleteDebugSession)
	s.mux.HandleFunc("POST /v1/debug-sessions/{session_id}/agents/run", s.handleRunDebugAgent)
	s.mux.HandleFunc("POST /v1/debug-sessions/{session_id}/workflows/run", s.handleRunDebugWorkflow)
	s.mux.HandleFunc("POST /v1/debug-sessions/{session_id}/agent-graphs/run", s.handleRunDebugAgentGraph)
}

func (s *Server) handleListDebugSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.debugSessions.ListSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sessions})
}

func (s *Server) handleCreateDebugSession(w http.ResponseWriter, r *http.Request) {
	if s.platformConfig == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("platform config service is required for debug sessions"))
		return
	}
	var req createDebugSessionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	preview, previewInfo, err := s.platformConfig.BuildPreviewBundle(r.Context(), req.BundleID, req.Entrypoint)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	session, err := s.debugSessions.CreateSession(r.Context(), preview, req.Entrypoint)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, debugSessionResponse{Session: session, Preview: previewInfo})
}

func (s *Server) handleGetDebugSession(w http.ResponseWriter, r *http.Request) {
	session, err := s.debugSessions.GetSession(r.Context(), r.PathValue("session_id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) handleDeleteDebugSession(w http.ResponseWriter, r *http.Request) {
	if err := s.debugSessions.DeleteSession(r.Context(), r.PathValue("session_id")); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handleRunDebugAgent(w http.ResponseWriter, r *http.Request) {
	var req runAgentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	startedAt := time.Now().UTC()
	appReq := app.AgentRequest{
		AgentID:      req.AgentID,
		UserMessage:  req.UserMessage,
		Conversation: req.Conversation,
		Context:      req.Context,
		TraceID:      req.TraceID,
		TraceContext: req.TraceContext,
	}
	sessionID := r.PathValue("session_id")
	result, err := s.debugSessions.RunAgent(r.Context(), sessionID, appReq)
	s.recordAgentRun(sessionID, startedAt, appReq, result, err)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, runAgentResponse{Result: toAgentResultResponse(result)})
}

func (s *Server) handleRunDebugWorkflow(w http.ResponseWriter, r *http.Request) {
	var req runWorkflowRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	startedAt := time.Now().UTC()
	appReq := app.WorkflowRequest{
		WorkflowID:   req.WorkflowID,
		Input:        req.Input,
		TraceContext: req.TraceContext,
	}
	sessionID := r.PathValue("session_id")
	result, err := s.debugSessions.RunWorkflow(r.Context(), sessionID, appReq)
	s.recordWorkflowRun(sessionID, startedAt, appReq, result, err)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"instance_id": result.Instance.InstanceID,
		"status":      result.Instance.Status,
		"output":      result.Output,
	})
}

func (s *Server) handleRunDebugAgentGraph(w http.ResponseWriter, r *http.Request) {
	var req runAgentGraphRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	startedAt := time.Now().UTC()
	appReq := app.AgentGraphRequest{
		AgentFlowID:  req.AgentFlowID,
		Input:        req.Input,
		TraceContext: req.TraceContext,
	}
	sessionID := r.PathValue("session_id")
	result, err := s.debugSessions.RunAgentGraph(r.Context(), sessionID, appReq)
	s.recordAgentGraphRun(sessionID, startedAt, appReq, result, err)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, agentGraphRunResponse(result))
}

type createDebugSessionRequest struct {
	BundleID   string                      `json:"bundle_id"`
	Entrypoint coreconfig.BundleEntrypoint `json:"entrypoint"`
}

type debugSessionResponse struct {
	Session app.DebugSessionSnapshot          `json:"session"`
	Preview platformconfig.BundleSnapshotInfo `json:"preview"`
}
