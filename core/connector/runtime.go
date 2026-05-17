package connector

import (
	"context"
	"fmt"
	"time"

	"flow-anything/core/runtimecontext"
)

// Runtime resolves OperationSpec + ConnectorSpec, dispatches to protocol
// executors, applies common policy, and emits lifecycle events.
type Runtime struct {
	repository Repository
	executors  *ExecutorRegistry
	hooks      []ConnectorEventHook
	callIDFn   func() string
	nowFn      func() time.Time
}

type RuntimeOption func(*Runtime)

func WithEventHook(hook ConnectorEventHook) RuntimeOption {
	return func(r *Runtime) {
		if hook != nil {
			r.hooks = append(r.hooks, hook)
		}
	}
}

func WithEventSink(sink ConnectorEventHook) RuntimeOption {
	return WithEventHook(sink)
}

func WithCallIDFunc(fn func() string) RuntimeOption {
	return func(r *Runtime) { r.callIDFn = fn }
}

func WithNowFunc(fn func() time.Time) RuntimeOption {
	return func(r *Runtime) { r.nowFn = fn }
}

func NewRuntime(repository Repository, executors *ExecutorRegistry, opts ...RuntimeOption) *Runtime {
	runtime := &Runtime{
		repository: repository,
		executors:  executors,
		callIDFn:   func() string { return fmt.Sprintf("conncall_%d", time.Now().UnixNano()) },
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

func (r *Runtime) Invoke(ctx context.Context, call InvokeRequest) (InvokeResult, error) {
	if r.repository == nil {
		return InvokeResult{}, fmt.Errorf("connector repository is required")
	}
	if call.OperationID == "" {
		return InvokeResult{}, fmt.Errorf("operation_id is required")
	}
	if call.CallID == "" {
		call.CallID = r.callIDFn()
	}
	traceContext := buildConnectorTraceContext(ctx, call)
	call.TraceID = traceContext.TraceID
	call.TraceContext = traceContext
	ctx = runtimecontext.WithTraceContext(ctx, traceContext)
	operation, ok := r.repository.GetOperation(call.OperationID)
	if !ok {
		return r.fail(ctx, call, ConnectorSpec{}, operation, "operation_not_found", fmt.Sprintf("operation %q not found", call.OperationID), time.Time{})
	}
	if !operation.Enabled {
		return r.fail(ctx, call, ConnectorSpec{}, operation, "operation_disabled", fmt.Sprintf("operation %q is disabled", call.OperationID), time.Time{})
	}
	connector, ok := r.repository.GetConnector(operation.ConnectorID)
	if !ok {
		return r.fail(ctx, call, connector, operation, "connector_not_found", fmt.Sprintf("connector %q not found", operation.ConnectorID), time.Time{})
	}
	if !connector.Enabled {
		return r.fail(ctx, call, connector, operation, "connector_disabled", fmt.Sprintf("connector %q is disabled", connector.ID), time.Time{})
	}
	executor, ok := r.executors.Get(connector.Protocol.Kind)
	if !ok {
		return r.fail(ctx, call, connector, operation, "executor_not_found", fmt.Sprintf("protocol executor %q not found", connector.Protocol.Kind), time.Time{})
	}
	if err := executor.ValidateConnector(connector); err != nil {
		return r.fail(ctx, call, connector, operation, "invalid_connector", err.Error(), time.Time{})
	}
	if err := executor.ValidateOperation(connector, operation); err != nil {
		return r.fail(ctx, call, connector, operation, "invalid_operation", err.Error(), time.Time{})
	}

	startedAt := r.nowFn()
	r.emit(ctx, ConnectorEvent{Type: EventInvokeStarted, TraceContext: call.TraceContext, CallID: call.CallID, ConnectorID: connector.ID, OperationID: operation.ID, Protocol: connector.Protocol.Kind, Input: call.Input, TraceID: call.TraceID})
	execCtx := ctx
	cancel := func() {}
	if operation.Policy.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, operation.Policy.Timeout)
	}
	result, err := executor.Execute(execCtx, ProtocolRequest{
		Connector: connector,
		Operation: operation,
		Call:      call,
		Input:     call.Input,
	})
	cancel()
	if err != nil {
		return r.fail(ctx, call, connector, operation, "executor_failed", err.Error(), startedAt)
	}
	output := result.Output
	if output == nil {
		output = map[string]any{}
	}
	invokeResult := InvokeResult{
		CallID:      call.CallID,
		ConnectorID: connector.ID,
		OperationID: operation.ID,
		Success:     true,
		Output:      output,
		Raw:         result.Raw,
		StartedAt:   startedAt,
		FinishedAt:  r.nowFn(),
	}
	r.emit(ctx, ConnectorEvent{Type: EventInvokeCompleted, TraceContext: call.TraceContext, CallID: call.CallID, ConnectorID: connector.ID, OperationID: operation.ID, Protocol: connector.Protocol.Kind, Input: call.Input, Output: output, TraceID: call.TraceID})
	return invokeResult, nil
}

func (r *Runtime) fail(ctx context.Context, call InvokeRequest, connector ConnectorSpec, operation OperationSpec, code, message string, startedAt time.Time) (InvokeResult, error) {
	if startedAt.IsZero() {
		startedAt = r.nowFn()
	}
	errValue := ConnectorError{Code: code, Message: message}
	result := InvokeResult{
		CallID:      call.CallID,
		ConnectorID: connector.ID,
		OperationID: call.OperationID,
		Success:     false,
		Error:       errValue,
		StartedAt:   startedAt,
		FinishedAt:  r.nowFn(),
	}
	if call.TraceContext.TraceID == "" {
		call.TraceContext = buildConnectorTraceContext(ctx, call)
		call.TraceID = call.TraceContext.TraceID
	}
	r.emit(ctx, ConnectorEvent{Type: EventInvokeFailed, TraceContext: call.TraceContext, CallID: call.CallID, ConnectorID: connector.ID, OperationID: call.OperationID, Protocol: connector.Protocol.Kind, Input: call.Input, Error: errValue, TraceID: call.TraceID})
	return result, fmt.Errorf("%s: %s", code, message)
}

func (r *Runtime) emit(ctx context.Context, event ConnectorEvent) {
	if event.TraceContext.TraceID == "" {
		event.TraceContext = buildConnectorTraceContext(ctx, InvokeRequest{CallID: event.CallID, OperationID: event.OperationID, TraceID: event.TraceID})
	}
	event.TraceID = event.TraceContext.TraceID
	event.Timestamp = r.nowFn()
	eventCtx := runtimecontext.WithTraceContext(ctx, event.TraceContext)
	for _, hook := range r.hooks {
		hook.OnConnectorEvent(eventCtx, event)
	}
}

func buildConnectorTraceContext(ctx context.Context, call InvokeRequest) runtimecontext.TraceContext {
	inherited, _ := runtimecontext.TraceContextFrom(ctx)
	traceID := firstNonEmpty(call.TraceContext.TraceID, call.TraceID, inherited.TraceID, call.CallID)
	return runtimecontext.TraceContext{
		TraceID:       traceID,
		SpanID:        firstNonEmpty(call.TraceContext.SpanID, runtimecontext.ConnectorSpanID(traceID, call.CallID)),
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
