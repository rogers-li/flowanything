package agentcore

import (
	"time"

	"flow-anything/core/runtimecontext"
)

// AgentEventType is the stable event protocol emitted by Agent Core.
//
// Trace, audit, metrics, and live progress should be implemented by upper
// layers through AgentEventHook listeners instead of being embedded in the
// reasoning core itself.
type AgentEventType string

const (
	EventAgentStarted         AgentEventType = "agent.started"
	EventAgentCompleted       AgentEventType = "agent.completed"
	EventAgentFailed          AgentEventType = "agent.failed"
	EventModelStarted         AgentEventType = "model.started"
	EventModelCompleted       AgentEventType = "model.completed"
	EventModelFailed          AgentEventType = "model.failed"
	EventPlanningStarted      AgentEventType = "planning.started"
	EventPlanningCompleted    AgentEventType = "planning.completed"
	EventPlanningFailed       AgentEventType = "planning.failed"
	EventCapabilityStarted    AgentEventType = "capability.started"
	EventCapabilityCompleted  AgentEventType = "capability.completed"
	EventCapabilityFailed     AgentEventType = "capability.failed"
	EventFinalAnswerStarted   AgentEventType = "final_answer.started"
	EventFinalAnswerCompleted AgentEventType = "final_answer.completed"
	EventFinalAnswerFailed    AgentEventType = "final_answer.failed"
)

// AgentEvent is a lifecycle event emitted from important reasoning points.
type AgentEvent struct {
	Type           AgentEventType
	TraceContext   runtimecontext.TraceContext
	TraceID        string
	AgentID        string
	Strategy       string
	CapabilityID   string
	CapabilityType string
	Data           map[string]any
	Error          string
	Timestamp      time.Time
}

// AgentEventHook receives Agent Core events. Hook failures are intentionally
// not returned to the agent runtime; event consumers should handle their own
// durability and retry semantics outside the core reasoning path.
type AgentEventHook interface {
	OnAgentEvent(ctx Context, event AgentEvent)
}

// AgentEventHookFunc adapts a function into an AgentEventHook.
type AgentEventHookFunc func(ctx Context, event AgentEvent)

func (fn AgentEventHookFunc) OnAgentEvent(ctx Context, event AgentEvent) {
	fn(ctx, event)
}

// AgentEventPublisher is passed to reasoning strategies so they can publish
// domain events without knowing how upper layers consume them.
type AgentEventPublisher interface {
	PublishAgentEvent(ctx Context, event AgentEvent)
}

type noopAgentEventPublisher struct{}

func (noopAgentEventPublisher) PublishAgentEvent(Context, AgentEvent) {}

// MemoryEventSink is useful for tests and local debugging. Production trace
// modules can implement AgentEventHook and transform events into spans.
type MemoryEventSink struct {
	Events []AgentEvent
}

func (s *MemoryEventSink) OnAgentEvent(_ Context, event AgentEvent) {
	s.Events = append(s.Events, event)
}
