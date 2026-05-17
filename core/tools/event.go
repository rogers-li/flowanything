package tools

import (
	"context"
	"time"

	"flow-anything/core/runtimecontext"
)

type ToolEventType string

const (
	EventToolStarted   ToolEventType = "tool.started"
	EventToolCompleted ToolEventType = "tool.completed"
	EventToolFailed    ToolEventType = "tool.failed"
)

// ToolEvent is emitted from Tool Core lifecycle points. Tracing, progress,
// audit, and metrics are upper-layer listeners.
type ToolEvent struct {
	Type         ToolEventType
	TraceContext runtimecontext.TraceContext
	CallID       string
	ToolID       string
	ToolType     ToolType
	Kind         string
	Input        map[string]any
	Output       map[string]any
	Error        ToolError
	TraceID      string
	Timestamp    time.Time
}

type ToolEventHook interface {
	OnToolEvent(ctx context.Context, event ToolEvent)
}

type ToolEventHookFunc func(ctx context.Context, event ToolEvent)

func (fn ToolEventHookFunc) OnToolEvent(ctx context.Context, event ToolEvent) {
	fn(ctx, event)
}

type MemoryEventSink struct {
	Events []ToolEvent
}

func (s *MemoryEventSink) OnToolEvent(_ context.Context, event ToolEvent) {
	s.Events = append(s.Events, event)
}
