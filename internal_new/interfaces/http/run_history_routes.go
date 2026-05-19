package httpapi

import (
	"context"
	"net/http"
	"time"

	coreconfig "flow-anything/core/config"
	"flow-anything/internal_new/app"
)

func (s *Server) registerRunHistoryRoutes() {
	s.mux.HandleFunc("GET /v1/run-history", s.handleListRunHistory)
	s.mux.HandleFunc("GET /v1/run-history/{run_id}", s.handleGetRunHistory)
	s.mux.HandleFunc("POST /v1/run-history/{run_id}/replay", s.handleReplayRun)
}

func (s *Server) handleListRunHistory(w http.ResponseWriter, r *http.Request) {
	items, err := s.runHistory.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetRunHistory(w http.ResponseWriter, r *http.Request) {
	record, err := s.runHistory.Get(r.Context(), r.PathValue("run_id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": record})
}

func (s *Server) handleReplayRun(w http.ResponseWriter, r *http.Request) {
	record, err := s.runHistory.Get(r.Context(), r.PathValue("run_id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	replay, replayErr := app.ReplayRun(r.Context(), record, s.runtime, s.debugSessions)
	normalizeReplayResult(&replay)
	if replay.Type != "" {
		var appendErr error
		replay, appendErr = s.runHistory.AppendContext(r.Context(), replay)
		if appendErr != nil {
			writeError(w, http.StatusInternalServerError, appendErr)
			return
		}
	}
	if replayErr != nil {
		writeError(w, http.StatusBadRequest, replayErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": replay})
}

func normalizeReplayResult(record *app.RunRecord) {
	switch result := record.Result.(type) {
	case app.AgentResult:
		record.Result = toAgentResultResponse(result)
	case app.WorkflowResult:
		record.Result = workflowResultHistoryPayload(result)
	case app.AgentGraphResult:
		record.Result = agentGraphResultHistoryPayload(result)
	}
}

func (s *Server) recordAgentRun(sessionID string, startedAt time.Time, req app.AgentRequest, result app.AgentResult, err error) {
	if s.runHistory == nil {
		return
	}
	record := app.RunRecord{
		Type:      app.RunTypeAgent,
		SessionID: sessionID,
		TraceID:   req.TraceID,
		AgentRequest: &app.AgentRunHistoryRequest{
			AgentID:      req.AgentID,
			UserMessage:  req.UserMessage,
			Conversation: req.Conversation,
			Context:      req.Context,
			TraceID:      req.TraceID,
			TraceContext: req.TraceContext,
		},
		Result:     toAgentResultResponse(result),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	}
	s.fillRunBundleFields(&record, sessionID)
	if err != nil {
		record.Status = app.RunStatusFailed
		record.Error = err.Error()
	} else {
		record.Status = app.RunStatusSucceeded
	}
	s.runHistory.Append(record)
}

func (s *Server) recordWorkflowRun(sessionID string, startedAt time.Time, req app.WorkflowRequest, result app.WorkflowResult, err error) {
	if s.runHistory == nil {
		return
	}
	record := app.RunRecord{
		Type:      app.RunTypeWorkflow,
		SessionID: sessionID,
		WorkflowRequest: &app.WorkflowRunHistoryRequest{
			WorkflowID:   req.WorkflowID,
			Input:        req.Input,
			TraceContext: req.TraceContext,
		},
		Result:     workflowResultHistoryPayload(result),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	}
	if req.TraceContext.TraceID != "" {
		record.TraceID = req.TraceContext.TraceID
	}
	s.fillRunBundleFields(&record, sessionID)
	if err != nil {
		record.Status = app.RunStatusFailed
		record.Error = err.Error()
	} else {
		record.Status = app.RunStatusSucceeded
	}
	s.runHistory.Append(record)
}

func (s *Server) recordAgentGraphRun(sessionID string, startedAt time.Time, req app.AgentGraphRequest, result app.AgentGraphResult, err error) {
	if s.runHistory == nil {
		return
	}
	record := app.RunRecord{
		Type:      app.RunTypeAgentGraph,
		SessionID: sessionID,
		AgentGraphRequest: &app.AgentGraphRunHistoryRequest{
			AgentFlowID:  req.AgentFlowID,
			Input:        req.Input,
			TraceContext: req.TraceContext,
		},
		Result:     agentGraphResultHistoryPayload(result),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	}
	if req.TraceContext.TraceID != "" {
		record.TraceID = req.TraceContext.TraceID
	}
	s.fillRunBundleFields(&record, sessionID)
	if err != nil {
		record.Status = app.RunStatusFailed
		record.Error = err.Error()
	} else {
		record.Status = app.RunStatusSucceeded
	}
	s.runHistory.Append(record)
}

func workflowResultHistoryPayload(result app.WorkflowResult) map[string]any {
	return map[string]any{
		"instance_id": result.Instance.InstanceID,
		"status":      result.Instance.Status,
		"output":      result.Output,
	}
}

func agentGraphRunResponse(result app.AgentGraphResult) map[string]any {
	return agentGraphResultHistoryPayload(result)
}

func agentGraphResultHistoryPayload(result app.AgentGraphResult) map[string]any {
	return map[string]any{
		"instance_id":  result.InstanceID,
		"status":       result.Status,
		"output":       result.Output,
		"text":         result.Text,
		"root_node_id": result.RootNodeID,
	}
}

func (s *Server) fillRunBundleFields(record *app.RunRecord, sessionID string) {
	if sessionID != "" && s.debugSessions != nil {
		if session, err := s.debugSessions.GetSession(context.Background(), sessionID); err == nil {
			record.BundleID = session.BundleID
			record.SourceBundleID = session.SourceBundleID
			record.BundleVersion = session.Version
			record.BundleLifecycle = string(session.Lifecycle)
			record.ContentHash = session.ContentHash
			record.Entrypoint = session.Entrypoint
			return
		}
	}
	if s.runtimeManager != nil {
		snapshot := s.runtimeManager.Snapshot()
		record.BundleID = snapshot.BundleID
		record.SourceBundleID = snapshot.SourceBundleID
		record.BundleVersion = snapshot.Version
		record.BundleLifecycle = string(snapshot.Lifecycle)
		record.ContentHash = snapshot.ContentHash
		return
	}
	bundle := s.runtime.Catalog().Bundle
	record.BundleID = bundle.ID
	record.BundleVersion = bundle.Version
	record.BundleLifecycle = metadataString(bundle.Metadata, coreconfig.BundleMetadataLifecycle)
	record.SourceBundleID = metadataString(bundle.Metadata, coreconfig.BundleMetadataSourceBundleID)
	record.ContentHash = metadataString(bundle.Metadata, coreconfig.BundleMetadataContentHash)
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}
