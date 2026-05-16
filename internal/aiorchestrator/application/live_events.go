package application

import (
	"context"
	"strings"
	"time"

	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/runtimeevent"
)

const runtimeLiveTraceIDPayloadKey = "live_trace_id"

func (s *Service) emitRuntimeEvent(ctx context.Context, evt event.Event, typ runtimeevent.Type, message string, payload map[string]any) {
	if s.options.RuntimeEventSink == nil || evt.TraceID == "" {
		return
	}
	runtimeEvent := runtimeevent.Event{
		Type:      typ,
		TenantID:  evt.TenantID,
		TraceID:   evt.TraceID,
		EventID:   evt.ID,
		AgentID:   evt.AgentID,
		SessionID: evt.SessionID,
		Message:   message,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	s.publishRuntimeEvent(ctx, evt, runtimeEvent)
}

func (s *Service) emitRuntimeStepEvent(ctx context.Context, evt event.Event, typ runtimeevent.Type, stepID string, stepType string, name string, status string, message string, payload map[string]any) {
	if s.options.RuntimeEventSink == nil || evt.TraceID == "" {
		return
	}
	runtimeEvent := runtimeevent.Event{
		Type:      typ,
		TenantID:  evt.TenantID,
		TraceID:   evt.TraceID,
		EventID:   evt.ID,
		AgentID:   evt.AgentID,
		SessionID: evt.SessionID,
		StepID:    stepID,
		StepType:  stepType,
		Name:      name,
		Status:    status,
		Message:   message,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	s.publishRuntimeEvent(ctx, evt, runtimeEvent)
}

func (s *Service) publishRuntimeEvent(ctx context.Context, evt event.Event, runtimeEvent runtimeevent.Event) {
	if err := s.options.RuntimeEventSink.Publish(ctx, runtimeEvent); err != nil {
		s.logger.Debug("failed to publish runtime event", "trace_id", runtimeEvent.TraceID, "event_type", runtimeEvent.Type, "error", err)
	}
	liveTraceID := liveTraceIDFromEvent(evt)
	if liveTraceID == "" || liveTraceID == runtimeEvent.TraceID {
		return
	}
	aliasEvent := runtimeEvent
	aliasEvent.TraceID = liveTraceID
	aliasEvent.Payload = cloneRuntimeEventPayload(runtimeEvent.Payload)
	if aliasEvent.Payload == nil {
		aliasEvent.Payload = map[string]any{}
	}
	aliasEvent.Payload["source_trace_id"] = runtimeEvent.TraceID
	if err := s.options.RuntimeEventSink.Publish(ctx, aliasEvent); err != nil {
		s.logger.Debug("failed to publish runtime alias event", "trace_id", liveTraceID, "source_trace_id", runtimeEvent.TraceID, "event_type", runtimeEvent.Type, "error", err)
	}
}

func liveTraceIDFromEvent(evt event.Event) string {
	if evt.Payload == nil {
		return ""
	}
	value, _ := evt.Payload[runtimeLiveTraceIDPayloadKey].(string)
	return strings.TrimSpace(value)
}

func cloneRuntimeEventPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
