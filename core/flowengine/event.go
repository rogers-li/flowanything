package flowengine

import (
	"context"
	"time"

	"flow-anything/core/runtimecontext"
)

// FlowEventType is the stable event protocol emitted by the engine.
//
// Trace, audit, metrics, and live progress should be implemented by upper
// layers through FlowEventHook listeners instead of being embedded in the
// engine itself.
type FlowEventType string

const (
	EventFlowStarted      FlowEventType = "flow.started"
	EventFlowWaiting      FlowEventType = "flow.waiting"
	EventFlowResumed      FlowEventType = "flow.resumed"
	EventFlowCompleted    FlowEventType = "flow.completed"
	EventFlowFailed       FlowEventType = "flow.failed"
	EventNodeScheduled    FlowEventType = "node.scheduled"
	EventNodeStarted      FlowEventType = "node.started"
	EventNodeWaiting      FlowEventType = "node.waiting"
	EventNodeCompleted    FlowEventType = "node.completed"
	EventNodeFailed       FlowEventType = "node.failed"
	EventNextNodeSelected FlowEventType = "node.next_selected"
)

// FlowEvent is a lifecycle event emitted from important execution points.
type FlowEvent struct {
	Type         FlowEventType
	TraceContext runtimecontext.TraceContext
	RunID        string
	FlowID       string
	NodeID       string
	NodeType     string
	SourceNodeID string
	TargetNodeID string
	Input        map[string]any
	Output       map[string]any
	Data         map[string]any
	Error        string
	Timestamp    time.Time
}

// FlowEventHook receives engine events. Hook failures are intentionally not
// returned to the engine; event consumers should handle their own durability and
// retry semantics outside the core execution path.
type FlowEventHook interface {
	OnFlowEvent(ctx context.Context, event FlowEvent)
}

// FlowEventHookFunc adapts a function into a FlowEventHook.
type FlowEventHookFunc func(ctx context.Context, event FlowEvent)

func (fn FlowEventHookFunc) OnFlowEvent(ctx context.Context, event FlowEvent) {
	fn(ctx, event)
}

// MemoryEventSink is useful for tests and local debugging. Production trace
// modules can implement FlowEventHook and transform events into spans.
type MemoryEventSink struct {
	Events []FlowEvent
}

func (s *MemoryEventSink) OnFlowEvent(_ context.Context, event FlowEvent) {
	s.Events = append(s.Events, event)
}
