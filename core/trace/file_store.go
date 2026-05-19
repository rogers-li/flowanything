package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// FileStore persists trace spans into a local JSON file.
//
// It is intentionally simple and optimized for local development and single-node
// runtimes. Production deployments can replace it with an OTLP exporter or a
// database-backed Store without changing trace producers.
type FileStore struct {
	Path string
	mu   sync.Mutex
}

func NewFileStore(path string) *FileStore {
	return &FileStore{Path: path}
}

func (s *FileStore) UpsertSpan(_ context.Context, span Span) error {
	if span.TraceID == "" {
		return fmt.Errorf("trace id is required")
	}
	if span.SpanID == "" {
		return fmt.Errorf("span id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return err
	}
	bySpanID := state.Traces[span.TraceID]
	if bySpanID == nil {
		bySpanID = map[string]Span{}
		state.Traces[span.TraceID] = bySpanID
	}
	if existing, ok := bySpanID[span.SpanID]; ok {
		span = mergeSpan(existing, span)
	}
	bySpanID[span.SpanID] = span
	return s.save(state)
}

func (s *FileStore) GetTrace(ctx context.Context, traceID string) (Trace, error) {
	spans, err := s.ListSpans(ctx, traceID)
	if err != nil {
		return Trace{}, err
	}
	return Trace{TraceID: traceID, Spans: spans}, nil
}

func (s *FileStore) ListSpans(_ context.Context, traceID string) ([]Span, error) {
	if traceID == "" {
		return nil, fmt.Errorf("trace id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return nil, err
	}
	bySpanID := state.Traces[traceID]
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

type fileStoreState struct {
	Traces map[string]map[string]Span `json:"traces"`
}

func (s *FileStore) load() (fileStoreState, error) {
	if s.Path == "" {
		return fileStoreState{}, fmt.Errorf("trace store path is required")
	}
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return fileStoreState{Traces: map[string]map[string]Span{}}, nil
	}
	if err != nil {
		return fileStoreState{}, err
	}
	if len(data) == 0 {
		return fileStoreState{Traces: map[string]map[string]Span{}}, nil
	}
	var state fileStoreState
	if err := json.Unmarshal(data, &state); err != nil {
		return fileStoreState{}, err
	}
	if state.Traces == nil {
		state.Traces = map[string]map[string]Span{}
	}
	return state, nil
}

func (s *FileStore) save(state fileStoreState) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o644)
}

func sortSpans(spans []Span) {
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].StartedAt.Equal(spans[j].StartedAt) {
			return spans[i].SpanID < spans[j].SpanID
		}
		return spans[i].StartedAt.Before(spans[j].StartedAt)
	})
}
