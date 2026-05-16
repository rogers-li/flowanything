package trace

import (
	"context"
	"fmt"
	"time"

	agentcore "flow-anything/core/agentcore"
	connectorcore "flow-anything/core/connector"
	"flow-anything/core/flowengine"
	"flow-anything/core/runtimecontext"
	toolscore "flow-anything/core/tools"
)

// Collector consumes events from flowengine, agentcore, tools, and connector
// packages and converts them into spans.
type Collector struct {
	store    Store
	redactor Redactor
	nowFn    func() time.Time
}

type CollectorOption func(*Collector)

func WithRedactor(redactor Redactor) CollectorOption {
	return func(c *Collector) {
		if redactor != nil {
			c.redactor = redactor
		}
	}
}

func WithNowFunc(fn func() time.Time) CollectorOption {
	return func(c *Collector) {
		if fn != nil {
			c.nowFn = fn
		}
	}
}

func NewCollector(store Store, opts ...CollectorOption) *Collector {
	if store == nil {
		store = NewMemoryStore()
	}
	collector := &Collector{
		store:    store,
		redactor: NewDefaultRedactor(),
		nowFn:    time.Now,
	}
	for _, opt := range opts {
		opt(collector)
	}
	return collector
}

func (c *Collector) Store() Store { return c.store }

func (c *Collector) OnFlowEvent(ctx context.Context, event flowengine.FlowEvent) {
	traceID := firstNonEmpty(event.TraceContext.TraceID, c.traceID(ctx, "", event.RunID))
	if traceID == "" {
		return
	}
	timestamp := c.timestamp(event.Timestamp)
	flowSpanID := firstNonEmpty(event.TraceContext.SpanID, flowSpanID(event.RunID))
	switch event.Type {
	case flowengine.EventFlowStarted:
		c.upsert(ctx, Span{
			TraceID: traceID, SpanID: flowSpanID, ParentSpanID: event.TraceContext.ParentSpanID,
			Name: "Flow " + event.FlowID, Kind: SpanKindFlow, Status: SpanStatusRunning,
			StartedAt: timestamp, Input: c.redact(event.Input),
			Attributes: map[string]any{"flow_id": event.FlowID, "run_id": event.RunID},
		})
	case flowengine.EventFlowWaiting:
		c.upsert(ctx, Span{TraceID: traceID, SpanID: flowSpanID, Status: SpanStatusWaiting, Events: []SpanEvent{spanEvent(string(event.Type), event.Data, timestamp)}})
	case flowengine.EventFlowResumed:
		c.upsert(ctx, Span{TraceID: traceID, SpanID: flowSpanID, Status: SpanStatusRunning, Events: []SpanEvent{spanEvent(string(event.Type), event.Data, timestamp)}})
	case flowengine.EventFlowCompleted:
		c.upsert(ctx, Span{TraceID: traceID, SpanID: flowSpanID, Status: SpanStatusOK, FinishedAt: timestamp, Output: c.redact(event.Output)})
	case flowengine.EventFlowFailed:
		c.upsert(ctx, Span{TraceID: traceID, SpanID: flowSpanID, Status: SpanStatusError, FinishedAt: timestamp, Error: event.Error})
	case flowengine.EventNodeStarted:
		spanID := firstNonEmpty(event.TraceContext.SpanID, nodeSpanID(event.RunID, event.NodeID))
		c.upsert(ctx, Span{
			TraceID: traceID, SpanID: spanID, ParentSpanID: firstNonEmpty(event.TraceContext.ParentSpanID, runtimecontext.FlowSpanID(event.RunID)),
			Name: "Node " + event.NodeID, Kind: SpanKindNode, Status: SpanStatusRunning,
			StartedAt: timestamp, Input: c.redact(event.Input),
			Attributes: mergeMap(map[string]any{"node_id": event.NodeID, "node_type": event.NodeType}, c.redact(event.Data)),
		})
	case flowengine.EventNodeWaiting:
		c.upsert(ctx, Span{TraceID: traceID, SpanID: firstNonEmpty(event.TraceContext.SpanID, nodeSpanID(event.RunID, event.NodeID)), Status: SpanStatusWaiting, Events: []SpanEvent{spanEvent(string(event.Type), event.Data, timestamp)}})
	case flowengine.EventNodeCompleted:
		c.upsert(ctx, Span{TraceID: traceID, SpanID: firstNonEmpty(event.TraceContext.SpanID, nodeSpanID(event.RunID, event.NodeID)), Status: SpanStatusOK, FinishedAt: timestamp, Output: c.redact(event.Output)})
	case flowengine.EventNodeFailed:
		c.upsert(ctx, Span{TraceID: traceID, SpanID: firstNonEmpty(event.TraceContext.SpanID, nodeSpanID(event.RunID, event.NodeID)), Status: SpanStatusError, FinishedAt: timestamp, Error: event.Error, Output: c.redact(event.Output)})
	case flowengine.EventNodeScheduled, flowengine.EventNextNodeSelected:
		parent := firstNonEmpty(event.TraceContext.SpanID, flowSpanID)
		if event.SourceNodeID != "" {
			parent = firstNonEmpty(event.TraceContext.SpanID, nodeSpanID(event.RunID, event.SourceNodeID))
		}
		c.upsert(ctx, Span{TraceID: traceID, SpanID: parent, Events: []SpanEvent{spanEvent(string(event.Type), map[string]any{"node_id": event.NodeID, "source_node_id": event.SourceNodeID, "target_node_id": event.TargetNodeID}, timestamp)}})
	}
}

func (c *Collector) OnAgentEvent(ctx agentcore.Context, event agentcore.AgentEvent) {
	contextValue, _ := ContextFrom(ctx)
	traceID := firstNonEmpty(event.TraceContext.TraceID, contextValue.TraceID, event.TraceID)
	if traceID == "" {
		traceID = "agent:" + event.AgentID
	}
	timestamp := c.timestamp(event.Timestamp)
	agentSpanID := firstNonEmpty(event.TraceContext.SpanID, agentSpanID(traceID, event.AgentID))
	parent := firstNonEmpty(event.TraceContext.ParentSpanID, contextValue.ParentSpanID, contextValue.SpanID)
	base := Span{TraceID: traceID, SpanID: agentSpanID, ParentSpanID: parent}
	switch event.Type {
	case agentcore.EventAgentStarted:
		base.Name = "Agent " + event.AgentID
		base.Kind = SpanKindAgent
		base.Status = SpanStatusRunning
		base.StartedAt = timestamp
		base.Attributes = map[string]any{"agent_id": event.AgentID, "strategy": event.Strategy}
	case agentcore.EventAgentCompleted:
		base.Status = SpanStatusOK
		base.FinishedAt = timestamp
		base.Output = c.redact(event.Data)
	case agentcore.EventAgentFailed:
		base.Status = SpanStatusError
		base.FinishedAt = timestamp
		base.Error = event.Error
	case agentcore.EventPlanningStarted, agentcore.EventPlanningCompleted, agentcore.EventPlanningFailed:
		c.handleChildAgentSpan(ctxFor(ctx), traceID, firstNonEmpty(event.TraceContext.SpanID, planningSpanID(traceID, event.AgentID)), firstNonEmpty(event.TraceContext.ParentSpanID, runtimecontext.AgentSpanID(traceID, event.AgentID)), SpanKindPlanning, "Planning", event.Type, event.Data, event.Error, timestamp)
		return
	case agentcore.EventModelStarted, agentcore.EventModelCompleted, agentcore.EventModelFailed:
		phase := fmt.Sprint(event.Data["phase"])
		if phase == "" || phase == "<nil>" {
			phase = "model"
		}
		c.handleChildAgentSpan(ctxFor(ctx), traceID, firstNonEmpty(event.TraceContext.SpanID, modelSpanID(traceID, event.AgentID, phase)), firstNonEmpty(event.TraceContext.ParentSpanID, runtimecontext.AgentSpanID(traceID, event.AgentID)), SpanKindLLM, "Model "+phase, event.Type, event.Data, event.Error, timestamp)
		return
	case agentcore.EventCapabilityStarted, agentcore.EventCapabilityCompleted, agentcore.EventCapabilityFailed:
		c.handleChildAgentSpan(ctxFor(ctx), traceID, firstNonEmpty(event.TraceContext.SpanID, capabilitySpanID(traceID, event.AgentID, event.CapabilityType, event.CapabilityID)), firstNonEmpty(event.TraceContext.ParentSpanID, runtimecontext.AgentSpanID(traceID, event.AgentID)), SpanKindTool, "Capability "+event.CapabilityID, event.Type, event.Data, event.Error, timestamp)
		return
	case agentcore.EventFinalAnswerStarted, agentcore.EventFinalAnswerCompleted, agentcore.EventFinalAnswerFailed:
		c.handleChildAgentSpan(ctxFor(ctx), traceID, firstNonEmpty(event.TraceContext.SpanID, finalAnswerSpanID(traceID, event.AgentID)), firstNonEmpty(event.TraceContext.ParentSpanID, runtimecontext.AgentSpanID(traceID, event.AgentID)), SpanKindLLM, "Final Answer", event.Type, event.Data, event.Error, timestamp)
		return
	default:
		base.Events = []SpanEvent{spanEvent(string(event.Type), event.Data, timestamp)}
	}
	c.upsert(ctxFor(ctx), base)
}

func (c *Collector) OnToolEvent(ctx context.Context, event toolscore.ToolEvent) {
	traceContext, _ := ContextFrom(ctx)
	traceID := firstNonEmpty(event.TraceContext.TraceID, traceContext.TraceID, event.TraceID, event.CallID)
	if traceID == "" {
		return
	}
	spanID := firstNonEmpty(event.TraceContext.SpanID, toolSpanID(traceID, event.CallID))
	timestamp := c.timestamp(event.Timestamp)
	span := Span{TraceID: traceID, SpanID: spanID}
	switch event.Type {
	case toolscore.EventToolStarted:
		span.ParentSpanID = firstNonEmpty(event.TraceContext.ParentSpanID, traceContext.ParentSpanID, traceContext.SpanID)
		span.Name = "Tool " + event.ToolID
		span.Kind = SpanKindTool
		span.Status = SpanStatusRunning
		span.StartedAt = timestamp
		span.Input = c.redact(event.Input)
		span.Attributes = map[string]any{"tool_id": event.ToolID, "tool_type": string(event.ToolType), "implementation_kind": event.Kind, "call_id": event.CallID}
	case toolscore.EventToolCompleted:
		span.Status = SpanStatusOK
		span.FinishedAt = timestamp
		span.Output = c.redact(event.Output)
	case toolscore.EventToolFailed:
		span.Status = SpanStatusError
		span.FinishedAt = timestamp
		span.Error = event.Error.Message
		span.Attributes = map[string]any{"error_code": event.Error.Code}
	}
	c.upsert(ctx, span)
}

func (c *Collector) OnConnectorEvent(ctx context.Context, event connectorcore.ConnectorEvent) {
	traceContext, _ := ContextFrom(ctx)
	traceID := firstNonEmpty(event.TraceContext.TraceID, traceContext.TraceID, event.TraceID, event.CallID)
	if traceID == "" {
		return
	}
	spanID := firstNonEmpty(event.TraceContext.SpanID, connectorSpanID(traceID, event.CallID))
	timestamp := c.timestamp(event.Timestamp)
	span := Span{TraceID: traceID, SpanID: spanID}
	switch event.Type {
	case connectorcore.EventInvokeStarted:
		span.ParentSpanID = firstNonEmpty(event.TraceContext.ParentSpanID, traceContext.ParentSpanID, traceContext.SpanID)
		span.Name = "Connector " + event.OperationID
		span.Kind = SpanKindConnector
		span.Status = SpanStatusRunning
		span.StartedAt = timestamp
		span.Input = c.redact(event.Input)
		span.Attributes = map[string]any{"connector_id": event.ConnectorID, "operation_id": event.OperationID, "protocol": event.Protocol, "call_id": event.CallID}
	case connectorcore.EventInvokeCompleted:
		span.Status = SpanStatusOK
		span.FinishedAt = timestamp
		span.Output = c.redact(event.Output)
	case connectorcore.EventInvokeFailed:
		span.Status = SpanStatusError
		span.FinishedAt = timestamp
		span.Error = event.Error.Message
		span.Attributes = map[string]any{"error_code": event.Error.Code}
	}
	c.upsert(ctx, span)
}

func (c *Collector) handleChildAgentSpan(ctx context.Context, traceID, spanID, parentID string, kind SpanKind, name string, eventType agentcore.AgentEventType, data map[string]any, errText string, timestamp time.Time) {
	span := Span{TraceID: traceID, SpanID: spanID, ParentSpanID: parentID, Name: name, Kind: kind}
	switch eventType {
	case agentcore.EventPlanningStarted, agentcore.EventModelStarted, agentcore.EventCapabilityStarted, agentcore.EventFinalAnswerStarted:
		span.Status = SpanStatusRunning
		span.StartedAt = timestamp
		span.Input = c.redact(data)
	case agentcore.EventPlanningCompleted, agentcore.EventModelCompleted, agentcore.EventCapabilityCompleted, agentcore.EventFinalAnswerCompleted:
		span.Status = SpanStatusOK
		span.FinishedAt = timestamp
		span.Output = c.redact(data)
	case agentcore.EventPlanningFailed, agentcore.EventModelFailed, agentcore.EventCapabilityFailed, agentcore.EventFinalAnswerFailed:
		span.Status = SpanStatusError
		span.FinishedAt = timestamp
		span.Error = errText
		span.Output = c.redact(data)
	}
	c.upsert(ctx, span)
}

func (c *Collector) upsert(ctx context.Context, span Span) {
	_ = c.store.UpsertSpan(ctx, span)
}

func (c *Collector) redact(value map[string]any) map[string]any {
	if c.redactor == nil {
		return value
	}
	return c.redactor.RedactMap(value)
}

func (c *Collector) timestamp(timestamp time.Time) time.Time {
	if timestamp.IsZero() {
		return c.nowFn()
	}
	return timestamp
}

func (c *Collector) traceID(ctx context.Context, explicit, fallback string) string {
	traceContext, _ := ContextFrom(ctx)
	return firstNonEmpty(traceContext.TraceID, explicit, fallback)
}

func (c *Collector) parentFromContext(ctx context.Context) string {
	traceContext, _ := ContextFrom(ctx)
	return firstNonEmpty(traceContext.ParentSpanID, traceContext.SpanID)
}

func spanEvent(name string, attributes map[string]any, timestamp time.Time) SpanEvent {
	return SpanEvent{Name: name, Attributes: attributes, Timestamp: timestamp}
}

func ctxFor(ctx agentcore.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	if standard, ok := ctx.(context.Context); ok {
		return standard
	}
	return context.Background()
}

func flowSpanID(runID string) string             { return runtimecontext.FlowSpanID(runID) }
func nodeSpanID(runID, nodeID string) string     { return runtimecontext.NodeSpanID(runID, nodeID) }
func agentSpanID(traceID, agentID string) string { return runtimecontext.AgentSpanID(traceID, agentID) }
func planningSpanID(traceID, agentID string) string {
	return runtimecontext.AgentPlanningSpanID(traceID, agentID)
}
func modelSpanID(traceID, agentID, phase string) string {
	return runtimecontext.AgentModelSpanID(traceID, agentID, phase)
}
func capabilitySpanID(traceID, agentID, capabilityType, capabilityID string) string {
	return runtimecontext.AgentCapabilitySpanID(traceID, agentID, capabilityType, capabilityID)
}
func finalAnswerSpanID(traceID, agentID string) string {
	return runtimecontext.AgentFinalAnswerSpanID(traceID, agentID)
}
func toolSpanID(traceID, callID string) string { return runtimecontext.ToolSpanID(traceID, callID) }
func connectorSpanID(traceID, callID string) string {
	return runtimecontext.ConnectorSpanID(traceID, callID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
