package connector

import (
	"context"
	"time"

	"flow-anything/core/runtimecontext"
)

type ConnectorEventType string

const (
	EventInvokeStarted   ConnectorEventType = "connector.invoke.started"
	EventInvokeCompleted ConnectorEventType = "connector.invoke.completed"
	EventInvokeFailed    ConnectorEventType = "connector.invoke.failed"
)

// ConnectorEvent is emitted from Connector Core lifecycle points. Trace,
// metrics, audit, and progress are upper-layer listeners.
type ConnectorEvent struct {
	Type         ConnectorEventType
	TraceContext runtimecontext.TraceContext
	CallID       string
	ConnectorID  string
	OperationID  string
	Protocol     string
	Input        map[string]any
	Output       map[string]any
	Error        ConnectorError
	TraceID      string
	Timestamp    time.Time
}

type ConnectorEventHook interface {
	OnConnectorEvent(ctx context.Context, event ConnectorEvent)
}

type ConnectorEventHookFunc func(ctx context.Context, event ConnectorEvent)

func (fn ConnectorEventHookFunc) OnConnectorEvent(ctx context.Context, event ConnectorEvent) {
	fn(ctx, event)
}

type MemoryEventSink struct {
	Events []ConnectorEvent
}

func (s *MemoryEventSink) OnConnectorEvent(_ context.Context, event ConnectorEvent) {
	s.Events = append(s.Events, event)
}
