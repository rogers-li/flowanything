package trace

import (
	"context"
	"testing"
	"time"
)

func TestFileStorePersistsTraceSpans(t *testing.T) {
	path := t.TempDir() + "/traces.json"
	store := NewFileStore(path)
	start := time.Unix(100, 0).UTC()
	if err := store.UpsertSpan(context.Background(), Span{
		TraceID:   "trace_file",
		SpanID:    "span_1",
		Name:      "Agent",
		Kind:      SpanKindAgent,
		Status:    SpanStatusRunning,
		StartedAt: start,
		Input:     map[string]any{"message": "hello"},
	}); err != nil {
		t.Fatalf("upsert span: %v", err)
	}
	if err := store.UpsertSpan(context.Background(), Span{
		TraceID:    "trace_file",
		SpanID:     "span_1",
		Status:     SpanStatusOK,
		FinishedAt: start.Add(time.Second),
		Output:     map[string]any{"answer": "world"},
	}); err != nil {
		t.Fatalf("merge span: %v", err)
	}

	reopened := NewFileStore(path)
	trace, err := reopened.GetTrace(context.Background(), "trace_file")
	if err != nil {
		t.Fatalf("get trace: %v", err)
	}
	if len(trace.Spans) != 1 {
		t.Fatalf("expected one span, got %#v", trace.Spans)
	}
	span := trace.Spans[0]
	if span.Name != "Agent" || span.Status != SpanStatusOK || span.Output["answer"] != "world" {
		t.Fatalf("unexpected persisted span: %#v", span)
	}
	if span.Input["message"] != "hello" {
		t.Fatalf("expected merged input to be preserved: %#v", span.Input)
	}
}
