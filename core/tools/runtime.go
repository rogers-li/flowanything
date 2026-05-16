package tools

import (
	"context"
	"fmt"
	"time"

	"flow-anything/core/runtimecontext"
)

// Runtime resolves ToolSpec, dispatches to the matching implementation
// executor, and emits lifecycle events.
type Runtime struct {
	repository ToolRepository
	executors  *ExecutorRegistry
	hooks      []ToolEventHook
	callIDFn   func() string
	nowFn      func() time.Time
}

type RuntimeOption func(*Runtime)

func WithEventHook(hook ToolEventHook) RuntimeOption {
	return func(r *Runtime) {
		if hook != nil {
			r.hooks = append(r.hooks, hook)
		}
	}
}

func WithEventSink(sink ToolEventHook) RuntimeOption {
	return WithEventHook(sink)
}

func WithCallIDFunc(fn func() string) RuntimeOption {
	return func(r *Runtime) { r.callIDFn = fn }
}

func WithNowFunc(fn func() time.Time) RuntimeOption {
	return func(r *Runtime) { r.nowFn = fn }
}

func NewRuntime(repository ToolRepository, executors *ExecutorRegistry, opts ...RuntimeOption) *Runtime {
	runtime := &Runtime{
		repository: repository,
		executors:  executors,
		callIDFn:   func() string { return fmt.Sprintf("toolcall_%d", time.Now().UnixNano()) },
		nowFn:      time.Now,
	}
	if runtime.executors == nil {
		runtime.executors = NewExecutorRegistry()
	}
	for _, opt := range opts {
		opt(runtime)
	}
	return runtime
}

func (r *Runtime) Invoke(ctx context.Context, call ToolCall) (ToolResult, error) {
	if r.repository == nil {
		return ToolResult{}, fmt.Errorf("tool repository is required")
	}
	if call.ToolID == "" {
		return ToolResult{}, fmt.Errorf("tool_id is required")
	}
	if call.CallID == "" {
		call.CallID = r.callIDFn()
	}
	traceContext := buildToolTraceContext(ctx, call)
	call.TraceID = traceContext.TraceID
	call.TraceContext = traceContext
	ctx = runtimecontext.WithTraceContext(ctx, traceContext)
	tool, ok := r.repository.GetTool(call.ToolID)
	if !ok {
		return r.fail(ctx, call, ToolSpec{}, "not_found", fmt.Sprintf("tool %q not found", call.ToolID), time.Time{})
	}
	if !tool.Enabled {
		return r.fail(ctx, call, tool, "disabled", fmt.Sprintf("tool %q is disabled", call.ToolID), time.Time{})
	}
	executor, ok := r.executors.Get(tool.Implementation.Kind)
	if !ok {
		return r.fail(ctx, call, tool, "executor_not_found", fmt.Sprintf("tool executor %q not found", tool.Implementation.Kind), time.Time{})
	}
	if err := executor.Validate(tool); err != nil {
		return r.fail(ctx, call, tool, "invalid_tool", err.Error(), time.Time{})
	}

	startedAt := r.nowFn()
	r.emit(ctx, ToolEvent{Type: EventToolStarted, TraceContext: call.TraceContext, CallID: call.CallID, ToolID: tool.ID, ToolType: tool.Type, Kind: tool.Implementation.Kind, Input: call.Input, TraceID: call.TraceID})
	execCtx := ctx
	cancel := func() {}
	if tool.Policy.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, tool.Policy.Timeout)
	}
	result, err := executor.Execute(execCtx, ToolExecutionRequest{Tool: tool, Call: call, Input: call.Input})
	cancel()
	if err != nil {
		return r.fail(ctx, call, tool, "executor_failed", err.Error(), startedAt)
	}
	toolResult := ToolResult{
		CallID:     call.CallID,
		ToolID:     tool.ID,
		Success:    true,
		Output:     result.Output,
		Raw:        result.Raw,
		StartedAt:  startedAt,
		FinishedAt: r.nowFn(),
	}
	r.emit(ctx, ToolEvent{Type: EventToolCompleted, TraceContext: call.TraceContext, CallID: call.CallID, ToolID: tool.ID, ToolType: tool.Type, Kind: tool.Implementation.Kind, Input: call.Input, Output: result.Output, TraceID: call.TraceID})
	return toolResult, nil
}

func (r *Runtime) fail(ctx context.Context, call ToolCall, tool ToolSpec, code, message string, startedAt time.Time) (ToolResult, error) {
	if startedAt.IsZero() {
		startedAt = r.nowFn()
	}
	errValue := ToolError{Code: code, Message: message}
	result := ToolResult{
		CallID:     call.CallID,
		ToolID:     call.ToolID,
		Success:    false,
		Error:      errValue,
		StartedAt:  startedAt,
		FinishedAt: r.nowFn(),
	}
	if call.TraceContext.TraceID == "" {
		call.TraceContext = buildToolTraceContext(ctx, call)
		call.TraceID = call.TraceContext.TraceID
	}
	r.emit(ctx, ToolEvent{Type: EventToolFailed, TraceContext: call.TraceContext, CallID: call.CallID, ToolID: call.ToolID, ToolType: tool.Type, Kind: tool.Implementation.Kind, Input: call.Input, Error: errValue, TraceID: call.TraceID})
	return result, fmt.Errorf("%s: %s", code, message)
}

func (r *Runtime) emit(ctx context.Context, event ToolEvent) {
	if event.TraceContext.TraceID == "" {
		event.TraceContext = buildToolTraceContext(ctx, ToolCall{CallID: event.CallID, ToolID: event.ToolID, TraceID: event.TraceID})
	}
	event.TraceID = event.TraceContext.TraceID
	event.Timestamp = r.nowFn()
	eventCtx := runtimecontext.WithTraceContext(ctx, event.TraceContext)
	for _, hook := range r.hooks {
		hook.OnToolEvent(eventCtx, event)
	}
}

func buildToolTraceContext(ctx context.Context, call ToolCall) runtimecontext.TraceContext {
	inherited, _ := runtimecontext.TraceContextFrom(ctx)
	traceID := firstNonEmpty(call.TraceContext.TraceID, call.TraceID, inherited.TraceID, call.CallID)
	return runtimecontext.TraceContext{
		TraceID:       traceID,
		SpanID:        firstNonEmpty(call.TraceContext.SpanID, runtimecontext.ToolSpanID(traceID, call.CallID)),
		ParentSpanID:  firstNonEmpty(call.TraceContext.ParentSpanID, inherited.SpanID),
		CorrelationID: firstNonEmpty(call.TraceContext.CorrelationID, inherited.CorrelationID),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
