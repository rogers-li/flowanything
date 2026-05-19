package trace

import (
	"context"
	"fmt"
	"sync"
)

type Store interface {
	UpsertSpan(ctx context.Context, span Span) error
	GetTrace(ctx context.Context, traceID string) (Trace, error)
	ListSpans(ctx context.Context, traceID string) ([]Span, error)
}

type MemoryStore struct {
	mu    sync.Mutex
	spans map[string]map[string]Span
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{spans: map[string]map[string]Span{}}
}

func (s *MemoryStore) UpsertSpan(_ context.Context, span Span) error {
	if span.TraceID == "" {
		return fmt.Errorf("trace id is required")
	}
	if span.SpanID == "" {
		return fmt.Errorf("span id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bySpanID := s.spans[span.TraceID]
	if bySpanID == nil {
		bySpanID = map[string]Span{}
		s.spans[span.TraceID] = bySpanID
	}
	existing, exists := bySpanID[span.SpanID]
	if exists {
		span = mergeSpan(existing, span)
	}
	bySpanID[span.SpanID] = span
	return nil
}

func (s *MemoryStore) GetTrace(ctx context.Context, traceID string) (Trace, error) {
	spans, err := s.ListSpans(ctx, traceID)
	if err != nil {
		return Trace{}, err
	}
	return Trace{TraceID: traceID, Spans: spans}, nil
}

func (s *MemoryStore) ListSpans(_ context.Context, traceID string) ([]Span, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bySpanID := s.spans[traceID]
	if bySpanID == nil {
		return nil, fmt.Errorf("trace %q not found", traceID)
	}
	spans := make([]Span, 0, len(bySpanID))
	for _, span := range bySpanID {
		spans = append(spans, span)
	}
	sortSpans(spans)
	return spans, nil
}

func mergeSpan(existing, incoming Span) Span {
	out := existing
	if incoming.ParentSpanID != "" {
		out.ParentSpanID = incoming.ParentSpanID
	}
	if incoming.Name != "" {
		out.Name = incoming.Name
	}
	if incoming.Kind != "" {
		out.Kind = incoming.Kind
	}
	if incoming.Status != "" {
		out.Status = incoming.Status
	}
	if !incoming.StartedAt.IsZero() && (out.StartedAt.IsZero() || incoming.StartedAt.Before(out.StartedAt)) {
		out.StartedAt = incoming.StartedAt
	}
	if !incoming.FinishedAt.IsZero() {
		out.FinishedAt = incoming.FinishedAt
	}
	out.Attributes = mergeMap(out.Attributes, incoming.Attributes)
	out.Input = mergeMap(out.Input, incoming.Input)
	out.Output = mergeMap(out.Output, incoming.Output)
	out.Events = append(out.Events, incoming.Events...)
	out.Links = append(out.Links, incoming.Links...)
	if incoming.Error != "" {
		out.Error = incoming.Error
	}
	return out
}

func mergeMap(left, right map[string]any) map[string]any {
	if left == nil && right == nil {
		return nil
	}
	out := map[string]any{}
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}
