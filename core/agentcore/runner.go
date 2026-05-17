package agentcore

import (
	"context"
	"fmt"
	"time"

	"flow-anything/core/runtimecontext"
)

// Runner wires model, capability registry, strategy registry, and event hooks.
type Runner struct {
	model            ModelClient
	capabilities     CapabilityRegistry
	strategies       *StrategyRegistry
	contextAssembler ContextAssembler
	memory           MemoryProvider
	hooks            []AgentEventHook
}

type RunnerOption func(*Runner)

func WithCapabilities(registry CapabilityRegistry) RunnerOption {
	return func(r *Runner) { r.capabilities = registry }
}

func WithStrategies(registry *StrategyRegistry) RunnerOption {
	return func(r *Runner) { r.strategies = registry }
}

func WithContextAssembler(assembler ContextAssembler) RunnerOption {
	return func(r *Runner) {
		if assembler != nil {
			r.contextAssembler = assembler
		}
	}
}

func WithMemoryProvider(provider MemoryProvider) RunnerOption {
	return func(r *Runner) { r.memory = provider }
}

func WithEventHook(hook AgentEventHook) RunnerOption {
	return func(r *Runner) {
		if hook != nil {
			r.hooks = append(r.hooks, hook)
		}
	}
}

func WithEventHookFunc(hook func(Context, AgentEvent)) RunnerOption {
	return func(r *Runner) {
		if hook != nil {
			r.hooks = append(r.hooks, AgentEventHookFunc(hook))
		}
	}
}

func WithEventSink(sink AgentEventHook) RunnerOption {
	return WithEventHook(sink)
}

func NewRunner(model ModelClient, opts ...RunnerOption) *Runner {
	runner := &Runner{
		model:            model,
		capabilities:     NewMapCapabilityRegistry(),
		strategies:       NewDefaultStrategyRegistry(),
		contextAssembler: NewDefaultContextAssembler(),
	}
	for _, opt := range opts {
		opt(runner)
	}
	return runner
}

func (r *Runner) Run(ctx Context, req AgentRunRequest) (AgentRunResult, error) {
	if r.model == nil {
		return AgentRunResult{}, fmt.Errorf("model client is required")
	}
	if err := validateAgentSpec(req.Agent); err != nil {
		return AgentRunResult{}, err
	}
	ctx, req = prepareAgentTraceContext(ctx, req)
	req.Agent.Policy = normalizeAgentPolicy(req.Agent.Policy)
	mode := req.Agent.ReasoningMode
	if mode == "" {
		mode = DirectStrategy{}.Name()
	}
	strategy, ok := r.strategies.Get(mode)
	if !ok {
		return AgentRunResult{}, fmt.Errorf("reasoning strategy %q is not registered", mode)
	}

	r.publish(ctx, req, AgentEvent{Type: EventAgentStarted, Strategy: strategy.Name()})
	result, err := strategy.Run(ctx, StrategyRuntime{
		Model:            r.model,
		Capabilities:     r.capabilities,
		ContextAssembler: r.contextAssembler,
		Memory:           r.memory,
		Events:           agentEventPublisher{hooks: r.hooks, traceContext: req.TraceContext},
	}, req)
	if err != nil {
		r.publish(ctx, req, AgentEvent{Type: EventAgentFailed, Strategy: strategy.Name(), Error: err.Error()})
		return AgentRunResult{}, err
	}
	r.publish(ctx, req, AgentEvent{Type: EventAgentCompleted, Strategy: strategy.Name(), Data: map[string]any{"text": result.Text}})
	return result, nil
}

func (r *Runner) publish(ctx Context, req AgentRunRequest, event AgentEvent) {
	event.TraceID = req.TraceID
	event.AgentID = req.Agent.ID
	event.TraceContext = normalizeAgentEventTraceContext(event, req.TraceContext)
	event.Timestamp = time.Now()
	eventCtx := withAgentTraceContext(ctx, event.TraceContext)
	for _, hook := range r.hooks {
		hook.OnAgentEvent(eventCtx, event)
	}
}

type agentEventPublisher struct {
	hooks        []AgentEventHook
	traceContext runtimecontext.TraceContext
}

func (p agentEventPublisher) PublishAgentEvent(ctx Context, event AgentEvent) {
	event.TraceContext = normalizeAgentEventTraceContext(event, p.traceContext)
	if event.TraceID == "" {
		event.TraceID = event.TraceContext.TraceID
	}
	event.Timestamp = time.Now()
	eventCtx := withAgentTraceContext(ctx, event.TraceContext)
	for _, hook := range p.hooks {
		hook.OnAgentEvent(eventCtx, event)
	}
}

func newAgentEvent(req AgentRunRequest, eventType AgentEventType, data map[string]any, errText string) AgentEvent {
	return AgentEvent{
		Type:      eventType,
		TraceID:   req.TraceID,
		AgentID:   req.Agent.ID,
		Data:      data,
		Error:     errText,
		Timestamp: time.Now(),
	}
}

// StrategyRuntime provides controlled access to dependencies from strategies.
type StrategyRuntime struct {
	Model            ModelClient
	Capabilities     CapabilityRegistry
	ContextAssembler ContextAssembler
	Memory           MemoryProvider
	Events           AgentEventPublisher
}

func prepareAgentTraceContext(ctx Context, req AgentRunRequest) (Context, AgentRunRequest) {
	inherited, _ := runtimecontext.TraceContextFrom(ctx)
	traceID := firstNonEmpty(req.TraceContext.TraceID, req.TraceID, inherited.TraceID, "agent:"+req.Agent.ID)
	agentSpanID := runtimecontext.AgentSpanID(traceID, req.Agent.ID)
	parentSpanID := firstNonEmpty(req.TraceContext.ParentSpanID, inherited.SpanID)
	traceContext := runtimecontext.TraceContext{
		TraceID:       traceID,
		SpanID:        firstNonEmpty(req.TraceContext.SpanID, agentSpanID),
		ParentSpanID:  parentSpanID,
		CorrelationID: firstNonEmpty(req.TraceContext.CorrelationID, inherited.CorrelationID),
	}
	req.TraceID = traceID
	req.TraceContext = traceContext
	return withAgentTraceContext(ctx, traceContext), req
}

func normalizeAgentEventTraceContext(event AgentEvent, agentTrace runtimecontext.TraceContext) runtimecontext.TraceContext {
	traceContext := event.TraceContext
	if traceContext.TraceID == "" {
		traceContext.TraceID = firstNonEmpty(event.TraceID, agentTrace.TraceID)
	}
	if traceContext.CorrelationID == "" {
		traceContext.CorrelationID = agentTrace.CorrelationID
	}
	if traceContext.SpanID != "" {
		return traceContext
	}
	switch event.Type {
	case EventPlanningStarted, EventPlanningCompleted, EventPlanningFailed:
		traceContext.SpanID = runtimecontext.AgentPlanningSpanID(traceContext.TraceID, event.AgentID)
		traceContext.ParentSpanID = agentTrace.SpanID
	case EventModelStarted, EventModelCompleted, EventModelFailed:
		phase := "model"
		if event.Data != nil {
			if value := fmt.Sprint(event.Data["phase"]); value != "" && value != "<nil>" {
				phase = value
			}
		}
		traceContext.SpanID = runtimecontext.AgentModelSpanID(traceContext.TraceID, event.AgentID, phase)
		traceContext.ParentSpanID = agentTrace.SpanID
	case EventCapabilityStarted, EventCapabilityCompleted, EventCapabilityFailed:
		traceContext.SpanID = runtimecontext.AgentCapabilitySpanID(traceContext.TraceID, event.AgentID, event.CapabilityType, event.CapabilityID)
		traceContext.ParentSpanID = agentTrace.SpanID
	case EventContextAssembled, EventContextFailed:
		traceContext.SpanID = runtimecontext.AgentModelSpanID(traceContext.TraceID, event.AgentID, "context")
		traceContext.ParentSpanID = agentTrace.SpanID
	case EventFinalAnswerStarted, EventFinalAnswerCompleted, EventFinalAnswerFailed:
		traceContext.SpanID = runtimecontext.AgentFinalAnswerSpanID(traceContext.TraceID, event.AgentID)
		traceContext.ParentSpanID = agentTrace.SpanID
	default:
		traceContext.SpanID = agentTrace.SpanID
		traceContext.ParentSpanID = agentTrace.ParentSpanID
	}
	return traceContext
}

func withAgentTraceContext(ctx Context, traceContext runtimecontext.TraceContext) Context {
	if ctx == nil {
		return runtimecontext.WithTraceContext(context.Background(), traceContext)
	}
	if standard, ok := ctx.(context.Context); ok {
		return runtimecontext.WithTraceContext(standard, traceContext)
	}
	return ctx
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
