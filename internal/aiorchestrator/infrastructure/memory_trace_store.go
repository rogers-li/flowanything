package infrastructure

import (
	"context"
	"sync"
	"time"

	"flow-anything/internal/aiorchestrator/domain"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryTraceStore struct {
	mu     sync.RWMutex
	traces map[string]domain.TraceRecord
}

func NewMemoryTraceStore() *MemoryTraceStore {
	return &MemoryTraceStore{traces: map[string]domain.TraceRecord{}}
}

func (s *MemoryTraceStore) StartTrace(ctx context.Context, trace domain.TraceRecord) error {
	if trace.TraceID == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "trace_id is required")
	}
	if trace.Status == "" {
		trace.Status = domain.TraceStatusRunning
	}
	if trace.StartedAt.IsZero() {
		trace.StartedAt = time.Now().UTC()
	}
	if trace.Steps == nil {
		trace.Steps = []domain.TraceStep{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.traces[traceKey(trace.TenantID, trace.TraceID)] = cloneTrace(trace)
	return nil
}

func (s *MemoryTraceStore) AppendStep(ctx context.Context, tenantID tenant.ID, traceID string, step domain.TraceStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := traceKey(tenantID, traceID)
	trace, ok := s.traces[key]
	if !ok {
		return apperrors.New(apperrors.CodeNotFound, "trace not found")
	}
	if step.StartedAt.IsZero() {
		step.StartedAt = time.Now().UTC()
	}
	if step.FinishedAt.IsZero() {
		step.FinishedAt = step.StartedAt
	}
	trace.Steps = append(trace.Steps, step)
	s.traces[key] = trace
	return nil
}

func (s *MemoryTraceStore) FinishTrace(ctx context.Context, tenantID tenant.ID, traceID string, status domain.TraceStatus, errText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := traceKey(tenantID, traceID)
	trace, ok := s.traces[key]
	if !ok {
		return apperrors.New(apperrors.CodeNotFound, "trace not found")
	}
	if status == "" {
		status = domain.TraceStatusSucceeded
	}
	now := time.Now().UTC()
	trace.Status = status
	trace.FinishedAt = now
	trace.DurationMillis = now.Sub(trace.StartedAt).Milliseconds()
	trace.Error = errText
	s.traces[key] = trace
	return nil
}

func (s *MemoryTraceStore) GetTrace(ctx context.Context, tenantID tenant.ID, traceID string) (domain.TraceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trace, ok := s.traces[traceKey(tenantID, traceID)]
	if !ok {
		return domain.TraceRecord{}, apperrors.New(apperrors.CodeNotFound, "trace not found")
	}
	return cloneTrace(trace), nil
}

func traceKey(tenantID tenant.ID, traceID string) string {
	return tenantID.String() + "/" + traceID
}

func cloneTrace(trace domain.TraceRecord) domain.TraceRecord {
	cloned := trace
	cloned.Steps = make([]domain.TraceStep, len(trace.Steps))
	copy(cloned.Steps, trace.Steps)
	return cloned
}
