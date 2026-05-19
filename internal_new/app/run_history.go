package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"flow-anything/core/agentcore"
	"flow-anything/core/runtimecontext"
)

type RunType string

const (
	RunTypeAgent      RunType = "agent"
	RunTypeWorkflow   RunType = "workflow"
	RunTypeAgentGraph RunType = "agent_graph"
)

type RunStatus string

const (
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
)

type AgentRunHistoryRequest struct {
	AgentID      string                      `json:"agent_id"`
	UserMessage  string                      `json:"user_message"`
	Conversation []agentcore.Message         `json:"conversation,omitempty"`
	Context      map[string]any              `json:"context,omitempty"`
	TraceID      string                      `json:"trace_id,omitempty"`
	TraceContext runtimecontext.TraceContext `json:"trace_context,omitempty"`
}

func (r AgentRunHistoryRequest) ToAppRequest() AgentRequest {
	return AgentRequest{
		AgentID:      r.AgentID,
		UserMessage:  r.UserMessage,
		Conversation: r.Conversation,
		Context:      r.Context,
		TraceID:      r.TraceID,
		TraceContext: r.TraceContext,
	}
}

type WorkflowRunHistoryRequest struct {
	WorkflowID   string                      `json:"workflow_id"`
	Input        map[string]any              `json:"input,omitempty"`
	TraceContext runtimecontext.TraceContext `json:"trace_context,omitempty"`
}

func (r WorkflowRunHistoryRequest) ToAppRequest() WorkflowRequest {
	return WorkflowRequest{
		WorkflowID:   r.WorkflowID,
		Input:        r.Input,
		TraceContext: r.TraceContext,
	}
}

type AgentGraphRunHistoryRequest struct {
	AgentFlowID  string                      `json:"agent_flow_id"`
	Input        map[string]any              `json:"input,omitempty"`
	TraceContext runtimecontext.TraceContext `json:"trace_context,omitempty"`
}

func (r AgentGraphRunHistoryRequest) ToAppRequest() AgentGraphRequest {
	return AgentGraphRequest{
		AgentFlowID:  r.AgentFlowID,
		Input:        r.Input,
		TraceContext: r.TraceContext,
	}
}

type RunRecord struct {
	ID                string                       `json:"id"`
	Type              RunType                      `json:"type"`
	Status            RunStatus                    `json:"status"`
	SessionID         string                       `json:"session_id,omitempty"`
	BundleID          string                       `json:"bundle_id,omitempty"`
	SourceBundleID    string                       `json:"source_bundle_id,omitempty"`
	BundleVersion     string                       `json:"bundle_version,omitempty"`
	BundleLifecycle   string                       `json:"bundle_lifecycle,omitempty"`
	ContentHash       string                       `json:"content_hash,omitempty"`
	Entrypoint        DebugEntrypoint              `json:"entrypoint,omitempty"`
	TraceID           string                       `json:"trace_id,omitempty"`
	AgentRequest      *AgentRunHistoryRequest      `json:"agent_request,omitempty"`
	WorkflowRequest   *WorkflowRunHistoryRequest   `json:"workflow_request,omitempty"`
	AgentGraphRequest *AgentGraphRunHistoryRequest `json:"agent_graph_request,omitempty"`
	Result            any                          `json:"result,omitempty"`
	Error             string                       `json:"error,omitempty"`
	StartedAt         time.Time                    `json:"started_at"`
	FinishedAt        time.Time                    `json:"finished_at"`
}

type RunHistory struct {
	store RunHistoryStore
	nowFn func() time.Time
}

type RunHistoryStore interface {
	SaveRun(ctx context.Context, record RunRecord) error
	LoadRun(ctx context.Context, id string) (RunRecord, error)
	ListRuns(ctx context.Context) ([]RunRecord, error)
}

type MemoryRunHistoryStore struct {
	mu      sync.RWMutex
	records map[string]RunRecord
	order   []string
}

func NewMemoryRunHistoryStore() *MemoryRunHistoryStore {
	return &MemoryRunHistoryStore{records: map[string]RunRecord{}}
}

func (s *MemoryRunHistoryStore) SaveRun(_ context.Context, record RunRecord) error {
	if record.ID == "" {
		return fmt.Errorf("run id is required")
	}
	s.mu.Lock()
	if _, exists := s.records[record.ID]; !exists {
		s.order = append(s.order, record.ID)
	}
	s.records[record.ID] = record
	s.mu.Unlock()
	return nil
}

func (s *MemoryRunHistoryStore) LoadRun(_ context.Context, id string) (RunRecord, error) {
	if id == "" {
		return RunRecord{}, fmt.Errorf("run id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[id]
	if !ok {
		return RunRecord{}, fmt.Errorf("run %q not found", id)
	}
	return record, nil
}

func (s *MemoryRunHistoryStore) ListRuns(_ context.Context) ([]RunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RunRecord, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.records[id])
	}
	return out, nil
}

func NewRunHistory() *RunHistory {
	return NewRunHistoryWithStore(NewMemoryRunHistoryStore())
}

func NewRunHistoryWithStore(store RunHistoryStore) *RunHistory {
	if store == nil {
		store = NewMemoryRunHistoryStore()
	}
	return &RunHistory{
		store: store,
		nowFn: time.Now,
	}
}

func (h *RunHistory) Append(record RunRecord) RunRecord {
	record, _ = h.AppendContext(context.Background(), record)
	return record
}

func (h *RunHistory) AppendContext(ctx context.Context, record RunRecord) (RunRecord, error) {
	if record.ID == "" {
		record.ID = newRunID()
	}
	now := h.now().UTC()
	if record.StartedAt.IsZero() {
		record.StartedAt = now
	}
	if record.FinishedAt.IsZero() {
		record.FinishedAt = now
	}
	if record.Status == "" {
		record.Status = RunStatusSucceeded
		if record.Error != "" {
			record.Status = RunStatusFailed
		}
	}
	if err := h.store.SaveRun(ctx, record); err != nil {
		return RunRecord{}, err
	}
	return record, nil
}

func (h *RunHistory) List(ctx context.Context) ([]RunRecord, error) {
	out, err := h.store.ListRuns(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out, nil
}

func (h *RunHistory) Get(ctx context.Context, id string) (RunRecord, error) {
	return h.store.LoadRun(ctx, id)
}

func ReplayRun(ctx context.Context, record RunRecord, runtime runtimeRunner, debugSessions *DebugSessionManager) (RunRecord, error) {
	replay := record
	replay.ID = ""
	replay.StartedAt = time.Now().UTC()
	replay.FinishedAt = time.Time{}
	replay.Error = ""
	replay.Result = nil
	replay.Status = ""

	switch record.Type {
	case RunTypeAgent:
		if record.AgentRequest == nil {
			return RunRecord{}, fmt.Errorf("agent run request is missing")
		}
		result, err := replayAgent(ctx, record, runtime, debugSessions)
		replay.Result = result
		replay.FinishedAt = time.Now().UTC()
		if err != nil {
			replay.Status = RunStatusFailed
			replay.Error = err.Error()
			return replay, err
		}
		replay.Status = RunStatusSucceeded
		return replay, nil
	case RunTypeWorkflow:
		if record.WorkflowRequest == nil {
			return RunRecord{}, fmt.Errorf("workflow run request is missing")
		}
		result, err := replayWorkflow(ctx, record, runtime, debugSessions)
		replay.Result = result
		replay.FinishedAt = time.Now().UTC()
		if err != nil {
			replay.Status = RunStatusFailed
			replay.Error = err.Error()
			return replay, err
		}
		replay.Status = RunStatusSucceeded
		return replay, nil
	case RunTypeAgentGraph:
		if record.AgentGraphRequest == nil {
			return RunRecord{}, fmt.Errorf("agent graph run request is missing")
		}
		result, err := replayAgentGraph(ctx, record, runtime, debugSessions)
		replay.Result = result
		replay.FinishedAt = time.Now().UTC()
		if err != nil {
			replay.Status = RunStatusFailed
			replay.Error = err.Error()
			return replay, err
		}
		replay.Status = RunStatusSucceeded
		return replay, nil
	default:
		return RunRecord{}, fmt.Errorf("unsupported run type %q", record.Type)
	}
}

type runtimeRunner interface {
	RunAgent(context.Context, AgentRequest) (AgentResult, error)
	RunWorkflow(context.Context, WorkflowRequest) (WorkflowResult, error)
	RunAgentGraph(context.Context, AgentGraphRequest) (AgentGraphResult, error)
}

func replayAgent(ctx context.Context, record RunRecord, runtime runtimeRunner, debugSessions *DebugSessionManager) (AgentResult, error) {
	req := record.AgentRequest.ToAppRequest()
	if record.SessionID != "" {
		if debugSessions == nil {
			return AgentResult{}, fmt.Errorf("debug session manager is not configured")
		}
		return debugSessions.RunAgent(ctx, record.SessionID, req)
	}
	if runtime == nil {
		return AgentResult{}, fmt.Errorf("runtime is not configured")
	}
	return runtime.RunAgent(ctx, req)
}

func replayWorkflow(ctx context.Context, record RunRecord, runtime runtimeRunner, debugSessions *DebugSessionManager) (WorkflowResult, error) {
	req := record.WorkflowRequest.ToAppRequest()
	if record.SessionID != "" {
		if debugSessions == nil {
			return WorkflowResult{}, fmt.Errorf("debug session manager is not configured")
		}
		return debugSessions.RunWorkflow(ctx, record.SessionID, req)
	}
	if runtime == nil {
		return WorkflowResult{}, fmt.Errorf("runtime is not configured")
	}
	return runtime.RunWorkflow(ctx, req)
}

func replayAgentGraph(ctx context.Context, record RunRecord, runtime runtimeRunner, debugSessions *DebugSessionManager) (AgentGraphResult, error) {
	req := record.AgentGraphRequest.ToAppRequest()
	if record.SessionID != "" {
		if debugSessions == nil {
			return AgentGraphResult{}, fmt.Errorf("debug session manager is not configured")
		}
		return debugSessions.RunAgentGraph(ctx, record.SessionID, req)
	}
	if runtime == nil {
		return AgentGraphResult{}, fmt.Errorf("runtime is not configured")
	}
	return runtime.RunAgentGraph(ctx, req)
}

func (h *RunHistory) now() time.Time {
	if h.nowFn != nil {
		return h.nowFn()
	}
	return time.Now()
}

func newRunID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "run_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("run_%d", time.Now().UnixNano())
}
