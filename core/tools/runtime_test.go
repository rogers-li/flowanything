package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"flow-anything/core/runtimecontext"
)

func TestRuntimeInvokesToolExecutorAndPublishesEvents(t *testing.T) {
	repository := NewMemoryToolRepository()
	if err := repository.Register(ToolSpec{
		ID:          "tool_weather",
		Name:        "Weather",
		Type:        ToolTypeConnector,
		Description: "Query weather.",
		Implementation: ToolImplementation{
			Kind: "connector",
			Ref:  "connop_weather",
		},
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	executors := NewExecutorRegistry()
	var captured ToolExecutionRequest
	var capturedTrace runtimecontext.TraceContext
	if err := executors.Register(ToolFunc{
		ImplementationKind: "connector",
		ExecuteFunc: func(ctx Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
			captured = req
			capturedTrace, _ = runtimecontext.TraceContextFrom(ctx)
			return ToolExecutionResult{
				Output: map[string]any{"city": req.Input["city"], "weather": "sunny"},
				Raw:    map[string]any{"ok": true},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	events := &MemoryEventSink{}
	runtime := NewRuntime(
		repository,
		executors,
		WithEventSink(events),
		WithCallIDFunc(func() string { return "call_test" }),
		WithNowFunc(func() time.Time { return time.Unix(100, 0).UTC() }),
	)

	result, err := runtime.Invoke(context.Background(), ToolCall{
		ToolID:  "tool_weather",
		Input:   map[string]any{"city": "Shanghai"},
		TraceID: "trace_tools",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success result: %#v", result)
	}
	if captured.Tool.ID != "tool_weather" || captured.Tool.Implementation.Ref != "connop_weather" {
		t.Fatalf("unexpected captured request: %#v", captured)
	}
	if captured.Call.TraceContext.SpanID != runtimecontext.ToolSpanID("trace_tools", "call_test") {
		t.Fatalf("tool call should carry tool span: %#v", captured.Call.TraceContext)
	}
	if capturedTrace.SpanID != runtimecontext.ToolSpanID("trace_tools", "call_test") {
		t.Fatalf("tool executor ctx should carry tool span: %#v", capturedTrace)
	}
	if result.Output["weather"] != "sunny" {
		t.Fatalf("unexpected output: %#v", result.Output)
	}
	if len(events.Events) != 2 {
		t.Fatalf("expected started/completed events, got %#v", events.Events)
	}
	if events.Events[0].Type != EventToolStarted || events.Events[1].Type != EventToolCompleted {
		t.Fatalf("unexpected events: %#v", events.Events)
	}
	if events.Events[0].TraceContext.SpanID != runtimecontext.ToolSpanID("trace_tools", "call_test") {
		t.Fatalf("tool event should carry tool span: %#v", events.Events[0].TraceContext)
	}
}

func TestRuntimeRejectsDisabledTool(t *testing.T) {
	repository := NewMemoryToolRepository()
	_ = repository.Register(ToolSpec{
		ID:             "tool_disabled",
		Type:           ToolTypeNative,
		Implementation: ToolImplementation{Kind: "native"},
		Enabled:        false,
	})
	runtime := NewRuntime(repository, NewExecutorRegistry(), WithCallIDFunc(func() string { return "call_disabled" }))

	result, err := runtime.Invoke(context.Background(), ToolCall{ToolID: "tool_disabled"})
	if err == nil {
		t.Fatal("expected disabled tool error")
	}
	if result.Success || result.Error.Code != "disabled" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRuntimeReportsExecutorFailure(t *testing.T) {
	repository := NewMemoryToolRepository()
	_ = repository.Register(ToolSpec{
		ID:             "tool_native",
		Type:           ToolTypeNative,
		Implementation: ToolImplementation{Kind: "native"},
		Enabled:        true,
	})
	executors := NewExecutorRegistry()
	_ = executors.Register(ToolFunc{
		ImplementationKind: "native",
		ExecuteFunc: func(Context, ToolExecutionRequest) (ToolExecutionResult, error) {
			return ToolExecutionResult{}, errors.New("boom")
		},
	})
	events := &MemoryEventSink{}
	runtime := NewRuntime(repository, executors, WithEventSink(events), WithCallIDFunc(func() string { return "call_failed" }))

	result, err := runtime.Invoke(context.Background(), ToolCall{ToolID: "tool_native"})
	if err == nil {
		t.Fatal("expected executor error")
	}
	if result.Success || result.Error.Code != "executor_failed" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(events.Events) != 2 || events.Events[1].Type != EventToolFailed {
		t.Fatalf("expected failed event: %#v", events.Events)
	}
}

func TestRuntimeReportsMissingExecutor(t *testing.T) {
	repository := NewMemoryToolRepository()
	_ = repository.Register(ToolSpec{
		ID:             "tool_missing_executor",
		Type:           ToolTypeWorkflow,
		Implementation: ToolImplementation{Kind: "workflow"},
		Enabled:        true,
	})
	runtime := NewRuntime(repository, NewExecutorRegistry(), WithCallIDFunc(func() string { return "call_missing" }))

	result, err := runtime.Invoke(context.Background(), ToolCall{ToolID: "tool_missing_executor"})
	if err == nil {
		t.Fatal("expected missing executor error")
	}
	if result.Success || result.Error.Code != "executor_not_found" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
