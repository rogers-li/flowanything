package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"flow-anything/core/agentcore"
	coreconfig "flow-anything/core/config"
	"flow-anything/core/runtimecontext"
	coretrace "flow-anything/core/trace"
	"flow-anything/internal_new/app"
	"flow-anything/internal_new/platformconfig"
)

type runtimeHost interface {
	Catalog() coreconfig.RuntimeCatalog
	TraceStore() coretrace.Store
	RunAgent(context.Context, app.AgentRequest) (app.AgentResult, error)
	InvokeTool(context.Context, app.ToolRequest) (app.ToolResult, error)
	InvokeConnector(context.Context, app.ConnectorRequest) (app.ConnectorResult, error)
	RunWorkflow(context.Context, app.WorkflowRequest) (app.WorkflowResult, error)
	RunAgentGraph(context.Context, app.AgentGraphRequest) (app.AgentGraphResult, error)
}

type Server struct {
	runtime        runtimeHost
	runtimeManager *app.RuntimeManager
	debugSessions  *app.DebugSessionManager
	runHistory     *app.RunHistory
	platformConfig *platformconfig.Service
	mux            *http.ServeMux
}

type ServerOption func(*Server)

func WithPlatformConfig(service *platformconfig.Service) ServerOption {
	return func(server *Server) {
		server.platformConfig = service
	}
}

func WithRuntimeManager(manager *app.RuntimeManager) ServerOption {
	return func(server *Server) {
		server.runtimeManager = manager
	}
}

func WithDebugSessions(manager *app.DebugSessionManager) ServerOption {
	return func(server *Server) {
		server.debugSessions = manager
	}
}

func WithRunHistory(history *app.RunHistory) ServerOption {
	return func(server *Server) {
		server.runHistory = history
	}
}

func NewServer(runtime runtimeHost, opts ...ServerOption) *Server {
	server := &Server{
		runtime: runtime,
		mux:     http.NewServeMux(),
	}
	for _, opt := range opts {
		opt(server)
	}
	server.registerRoutes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /v1/catalog", s.handleCatalog)
	s.mux.HandleFunc("POST /v1/agents/run", s.handleRunAgent)
	s.mux.HandleFunc("POST /v1/tools/invoke", s.handleInvokeTool)
	s.mux.HandleFunc("POST /v1/connectors/invoke", s.handleInvokeConnector)
	s.mux.HandleFunc("POST /v1/workflows/run", s.handleRunWorkflow)
	s.mux.HandleFunc("POST /v1/agent-graphs/run", s.handleRunAgentGraph)
	s.mux.HandleFunc("GET /v1/traces/{trace_id}", s.handleGetTrace)
	if s.platformConfig != nil {
		s.registerPlatformConfigRoutes()
	}
	if s.runtimeManager != nil {
		s.registerRuntimeControlRoutes()
	}
	if s.debugSessions != nil {
		s.registerDebugSessionRoutes()
	}
	if s.runHistory != nil {
		s.registerRunHistoryRoutes()
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "internal_new"})
}

func (s *Server) handleCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.runtime.Catalog().Bundle)
}

func (s *Server) handleRunAgent(w http.ResponseWriter, r *http.Request) {
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
	result, err := s.runtime.RunAgent(r.Context(), appReq)
	s.recordAgentRun("", startedAt, appReq, result, err)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, runAgentResponse{Result: toAgentResultResponse(result)})
}

func (s *Server) handleInvokeTool(w http.ResponseWriter, r *http.Request) {
	var req invokeToolRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	result, err := s.runtime.InvokeTool(r.Context(), app.ToolRequest{
		CallID:       req.CallID,
		ToolID:       req.ToolID,
		Input:        req.Input,
		Metadata:     req.Metadata,
		TraceID:      req.TraceID,
		TraceContext: req.TraceContext,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (s *Server) handleInvokeConnector(w http.ResponseWriter, r *http.Request) {
	var req invokeConnectorRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	result, err := s.runtime.InvokeConnector(r.Context(), app.ConnectorRequest{
		CallID:       req.CallID,
		OperationID:  req.OperationID,
		Input:        req.Input,
		Metadata:     req.Metadata,
		TraceID:      req.TraceID,
		TraceContext: req.TraceContext,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
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
	result, err := s.runtime.RunWorkflow(r.Context(), appReq)
	s.recordWorkflowRun("", startedAt, appReq, result, err)
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

func (s *Server) handleRunAgentGraph(w http.ResponseWriter, r *http.Request) {
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
	result, err := s.runtime.RunAgentGraph(r.Context(), appReq)
	s.recordAgentGraphRun("", startedAt, appReq, result, err)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, agentGraphRunResponse(result))
}

func (s *Server) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("trace_id")
	traceValue, err := s.runtime.TraceStore().GetTrace(r.Context(), traceID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, toTraceResponse(traceValue))
}

type runAgentRequest struct {
	AgentID      string                      `json:"agent_id"`
	UserMessage  string                      `json:"user_message"`
	Conversation []agentcore.Message         `json:"conversation"`
	Context      map[string]any              `json:"context"`
	TraceID      string                      `json:"trace_id"`
	TraceContext runtimecontext.TraceContext `json:"trace_context"`
}

type runAgentResponse struct {
	Result agentResultResponse `json:"result"`
}

type agentResultResponse struct {
	Text    string                 `json:"text"`
	Output  map[string]any         `json:"output,omitempty"`
	Actions []actionResultResponse `json:"actions,omitempty"`
	Raw     any                    `json:"raw,omitempty"`
}

type actionResultResponse struct {
	Action agentcore.PlannedAction  `json:"action"`
	Result capabilityResultResponse `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type capabilityResultResponse struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Text   string         `json:"text,omitempty"`
	Output map[string]any `json:"output,omitempty"`
	Raw    any            `json:"raw,omitempty"`
}

func toAgentResultResponse(result app.AgentResult) agentResultResponse {
	actions := make([]actionResultResponse, 0, len(result.Actions))
	for _, action := range result.Actions {
		actions = append(actions, actionResultResponse{
			Action: action.Action,
			Result: capabilityResultResponse{
				ID:     action.Result.ID,
				Type:   action.Result.Type,
				Text:   action.Result.Text,
				Output: action.Result.Output,
				Raw:    action.Result.Raw,
			},
			Error: action.Error,
		})
	}
	return agentResultResponse{
		Text:    result.Text,
		Output:  result.Output,
		Actions: actions,
		Raw:     result.Raw,
	}
}

type invokeToolRequest struct {
	CallID       string                      `json:"call_id"`
	ToolID       string                      `json:"tool_id"`
	Input        map[string]any              `json:"input"`
	Metadata     map[string]any              `json:"metadata"`
	TraceID      string                      `json:"trace_id"`
	TraceContext runtimecontext.TraceContext `json:"trace_context"`
}

type invokeConnectorRequest struct {
	CallID       string                      `json:"call_id"`
	OperationID  string                      `json:"operation_id"`
	Input        map[string]any              `json:"input"`
	Metadata     map[string]any              `json:"metadata"`
	TraceID      string                      `json:"trace_id"`
	TraceContext runtimecontext.TraceContext `json:"trace_context"`
}

type runWorkflowRequest struct {
	WorkflowID   string                      `json:"workflow_id"`
	Input        map[string]any              `json:"input"`
	TraceContext runtimecontext.TraceContext `json:"trace_context"`
}

type runAgentGraphRequest struct {
	AgentFlowID  string                      `json:"agent_flow_id"`
	Input        map[string]any              `json:"input"`
	TraceContext runtimecontext.TraceContext `json:"trace_context"`
}

type traceResponse struct {
	Trace traceValueResponse     `json:"trace"`
	Tree  []spanTreeNodeResponse `json:"tree"`
}

type traceValueResponse struct {
	TraceID string         `json:"trace_id"`
	Spans   []spanResponse `json:"spans"`
}

type spanTreeNodeResponse struct {
	Span     spanResponse           `json:"span"`
	Children []spanTreeNodeResponse `json:"children,omitempty"`
}

type spanResponse struct {
	TraceID      string               `json:"trace_id"`
	SpanID       string               `json:"span_id"`
	ParentSpanID string               `json:"parent_span_id,omitempty"`
	Name         string               `json:"name"`
	Kind         coretrace.SpanKind   `json:"kind"`
	Status       coretrace.SpanStatus `json:"status"`
	StartedAt    time.Time            `json:"started_at,omitempty"`
	FinishedAt   time.Time            `json:"finished_at,omitempty"`
	Attributes   map[string]any       `json:"attributes,omitempty"`
	Events       []spanEventResponse  `json:"events,omitempty"`
	Input        map[string]any       `json:"input,omitempty"`
	Output       map[string]any       `json:"output,omitempty"`
	Error        string               `json:"error,omitempty"`
}

type spanEventResponse struct {
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Timestamp  time.Time      `json:"timestamp,omitempty"`
}

func toTraceResponse(traceValue coretrace.Trace) traceResponse {
	spans := make([]spanResponse, 0, len(traceValue.Spans))
	for _, span := range traceValue.Spans {
		spans = append(spans, toSpanResponse(span))
	}
	tree := coretrace.BuildTree(traceValue.Spans)
	return traceResponse{
		Trace: traceValueResponse{TraceID: traceValue.TraceID, Spans: spans},
		Tree:  toSpanTreeResponse(tree),
	}
}

func toSpanTreeResponse(nodes []coretrace.SpanTreeNode) []spanTreeNodeResponse {
	out := make([]spanTreeNodeResponse, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, spanTreeNodeResponse{
			Span:     toSpanResponse(node.Span),
			Children: toSpanTreeResponse(node.Children),
		})
	}
	return out
}

func toSpanResponse(span coretrace.Span) spanResponse {
	events := make([]spanEventResponse, 0, len(span.Events))
	for _, event := range span.Events {
		events = append(events, spanEventResponse{
			Name:       event.Name,
			Attributes: event.Attributes,
			Timestamp:  event.Timestamp,
		})
	}
	return spanResponse{
		TraceID:      span.TraceID,
		SpanID:       span.SpanID,
		ParentSpanID: span.ParentSpanID,
		Name:         span.Name,
		Kind:         span.Kind,
		Status:       span.Status,
		StartedAt:    span.StartedAt,
		FinishedAt:   span.FinishedAt,
		Attributes:   span.Attributes,
		Events:       events,
		Input:        span.Input,
		Output:       span.Output,
		Error:        span.Error,
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, statusCode int, err error) {
	writeJSON(w, statusCode, map[string]any{
		"error": map[string]string{
			"message": err.Error(),
		},
	})
}
