package runtimecontext

import "context"

type contextKey string

const traceContextKey contextKey = "flow-anything.runtime.trace-context"

// TraceContext is the shared propagation protocol across core runtime modules.
// It is intentionally kept outside the trace collector package so engines,
// agents, tools, and connectors can propagate trace identity without depending
// on a concrete tracing implementation.
type TraceContext struct {
	TraceID       string `json:"trace_id,omitempty"`
	SpanID        string `json:"span_id,omitempty"`
	ParentSpanID  string `json:"parent_span_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func WithTraceContext(ctx context.Context, traceContext TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey, traceContext)
}

func TraceContextFrom(ctx interface{ Value(key any) any }) (TraceContext, bool) {
	if ctx == nil {
		return TraceContext{}, false
	}
	value := ctx.Value(traceContextKey)
	traceContext, ok := value.(TraceContext)
	return traceContext, ok
}

func TraceIDFrom(ctx interface{ Value(key any) any }, fallbacks ...string) string {
	if traceContext, ok := TraceContextFrom(ctx); ok && traceContext.TraceID != "" {
		return traceContext.TraceID
	}
	for _, fallback := range fallbacks {
		if fallback != "" {
			return fallback
		}
	}
	return ""
}

func ChildTraceContext(parent TraceContext, spanID string) TraceContext {
	return TraceContext{
		TraceID:       parent.TraceID,
		SpanID:        spanID,
		ParentSpanID:  parent.SpanID,
		CorrelationID: parent.CorrelationID,
	}
}

func RootTraceContext(traceID, spanID string, parent TraceContext) TraceContext {
	return TraceContext{
		TraceID:       firstNonEmpty(traceID, parent.TraceID),
		SpanID:        spanID,
		ParentSpanID:  parent.SpanID,
		CorrelationID: parent.CorrelationID,
	}
}

func FlowSpanID(runID string) string {
	return "flow:" + runID
}

func NodeSpanID(runID, nodeID string) string {
	return FlowSpanID(runID) + ":node:" + nodeID
}

func AgentSpanID(traceID, agentID string) string {
	return "agent:" + traceID + ":" + agentID
}

func AgentPlanningSpanID(traceID, agentID string) string {
	return AgentSpanID(traceID, agentID) + ":planning"
}

func AgentModelSpanID(traceID, agentID, phase string) string {
	return AgentSpanID(traceID, agentID) + ":model:" + phase
}

func AgentCapabilitySpanID(traceID, agentID, capabilityType, capabilityID string) string {
	return AgentSpanID(traceID, agentID) + ":capability:" + capabilityType + ":" + capabilityID
}

func AgentFinalAnswerSpanID(traceID, agentID string) string {
	return AgentSpanID(traceID, agentID) + ":final_answer"
}

func ToolSpanID(traceID, callID string) string {
	return "tool:" + traceID + ":" + callID
}

func ConnectorSpanID(traceID, callID string) string {
	return "connector:" + traceID + ":" + callID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
