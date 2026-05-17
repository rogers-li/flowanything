package connector

import (
	"context"
	"errors"
	"testing"
	"time"

	"flow-anything/core/runtimecontext"
)

func TestRuntimeInvokesProtocolExecutorAndPublishesEvents(t *testing.T) {
	repository := NewMemoryRepository()
	if err := repository.RegisterConnector(ConnectorSpec{
		ID:      "conn_weather",
		Name:    "Weather Service",
		Enabled: true,
		Protocol: ProtocolSpec{
			Kind:    "http",
			BaseURL: "https://weather.example.com",
		},
		Auth: AuthSpec{Type: "api_key", SecretRef: "WEATHER_API_KEY"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RegisterOperation(OperationSpec{
		ID:          "op_query_weather",
		ConnectorID: "conn_weather",
		Name:        "Query Weather",
		Enabled:     true,
		Request: OperationRequest{
			Method: "GET",
			Path:   "/weather",
		},
	}); err != nil {
		t.Fatal(err)
	}

	executors := NewExecutorRegistry()
	var captured ProtocolRequest
	var capturedTrace runtimecontext.TraceContext
	if err := executors.Register(ProtocolFunc{
		ProtocolKind: "http",
		ValidateConnectorFunc: func(connector ConnectorSpec) error {
			if connector.Protocol.BaseURL == "" {
				t.Fatal("base url should be present")
			}
			return nil
		},
		ValidateOperationFunc: func(_ ConnectorSpec, operation OperationSpec) error {
			if operation.Request.Path == "" {
				t.Fatal("operation path should be present")
			}
			return nil
		},
		ExecuteFunc: func(ctx context.Context, req ProtocolRequest) (ProtocolResult, error) {
			captured = req
			capturedTrace, _ = runtimecontext.TraceContextFrom(ctx)
			return ProtocolResult{
				Output: map[string]any{"city": req.Input["city"], "weather": "sunny"},
				Raw:    map[string]any{"status_code": 200},
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
		WithCallIDFunc(func() string { return "conncall_test" }),
		WithNowFunc(func() time.Time { return time.Unix(100, 0).UTC() }),
	)
	result, err := runtime.Invoke(context.Background(), InvokeRequest{
		OperationID: "op_query_weather",
		Input:       map[string]any{"city": "Shanghai"},
		TraceID:     "trace_connector",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success result: %#v", result)
	}
	if captured.Connector.ID != "conn_weather" || captured.Operation.ID != "op_query_weather" {
		t.Fatalf("unexpected captured request: %#v", captured)
	}
	if captured.Call.TraceContext.SpanID != runtimecontext.ConnectorSpanID("trace_connector", "conncall_test") {
		t.Fatalf("connector call should carry connector span: %#v", captured.Call.TraceContext)
	}
	if capturedTrace.SpanID != runtimecontext.ConnectorSpanID("trace_connector", "conncall_test") {
		t.Fatalf("connector executor ctx should carry connector span: %#v", capturedTrace)
	}
	if result.Output["weather"] != "sunny" {
		t.Fatalf("unexpected output: %#v", result.Output)
	}
	if len(events.Events) != 2 {
		t.Fatalf("expected started/completed events, got %#v", events.Events)
	}
	if events.Events[0].Type != EventInvokeStarted || events.Events[1].Type != EventInvokeCompleted {
		t.Fatalf("unexpected events: %#v", events.Events)
	}
	if events.Events[0].TraceContext.SpanID != runtimecontext.ConnectorSpanID("trace_connector", "conncall_test") {
		t.Fatalf("connector event should carry connector span: %#v", events.Events[0].TraceContext)
	}
}

func TestRuntimeRejectsDisabledOperation(t *testing.T) {
	repository := NewMemoryRepository()
	_ = repository.RegisterOperation(OperationSpec{
		ID:          "op_disabled",
		ConnectorID: "conn_any",
		Enabled:     false,
	})
	runtime := NewRuntime(repository, NewExecutorRegistry(), WithCallIDFunc(func() string { return "call_disabled" }))

	result, err := runtime.Invoke(context.Background(), InvokeRequest{OperationID: "op_disabled"})
	if err == nil {
		t.Fatal("expected disabled operation error")
	}
	if result.Success || result.Error.Code != "operation_disabled" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRuntimeRejectsDisabledConnector(t *testing.T) {
	repository := NewMemoryRepository()
	_ = repository.RegisterConnector(ConnectorSpec{
		ID:       "conn_disabled",
		Enabled:  false,
		Protocol: ProtocolSpec{Kind: "http"},
	})
	_ = repository.RegisterOperation(OperationSpec{
		ID:          "op_enabled",
		ConnectorID: "conn_disabled",
		Enabled:     true,
	})
	runtime := NewRuntime(repository, NewExecutorRegistry(), WithCallIDFunc(func() string { return "call_connector_disabled" }))

	result, err := runtime.Invoke(context.Background(), InvokeRequest{OperationID: "op_enabled"})
	if err == nil {
		t.Fatal("expected disabled connector error")
	}
	if result.Success || result.Error.Code != "connector_disabled" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRuntimeReportsMissingExecutor(t *testing.T) {
	repository := NewMemoryRepository()
	_ = repository.RegisterConnector(ConnectorSpec{
		ID:       "conn_http",
		Enabled:  true,
		Protocol: ProtocolSpec{Kind: "http"},
	})
	_ = repository.RegisterOperation(OperationSpec{
		ID:          "op_http",
		ConnectorID: "conn_http",
		Enabled:     true,
	})
	runtime := NewRuntime(repository, NewExecutorRegistry(), WithCallIDFunc(func() string { return "call_missing_executor" }))

	result, err := runtime.Invoke(context.Background(), InvokeRequest{OperationID: "op_http"})
	if err == nil {
		t.Fatal("expected missing executor error")
	}
	if result.Success || result.Error.Code != "executor_not_found" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRuntimeReportsExecutorFailure(t *testing.T) {
	repository := NewMemoryRepository()
	_ = repository.RegisterConnector(ConnectorSpec{
		ID:       "conn_http",
		Enabled:  true,
		Protocol: ProtocolSpec{Kind: "http"},
	})
	_ = repository.RegisterOperation(OperationSpec{
		ID:          "op_http",
		ConnectorID: "conn_http",
		Enabled:     true,
	})
	executors := NewExecutorRegistry()
	_ = executors.Register(ProtocolFunc{
		ProtocolKind: "http",
		ExecuteFunc: func(context.Context, ProtocolRequest) (ProtocolResult, error) {
			return ProtocolResult{}, errors.New("boom")
		},
	})
	events := &MemoryEventSink{}
	runtime := NewRuntime(repository, executors, WithEventSink(events), WithCallIDFunc(func() string { return "call_failed" }))

	result, err := runtime.Invoke(context.Background(), InvokeRequest{OperationID: "op_http"})
	if err == nil {
		t.Fatal("expected executor error")
	}
	if result.Success || result.Error.Code != "executor_failed" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(events.Events) != 2 || events.Events[1].Type != EventInvokeFailed {
		t.Fatalf("expected failed event: %#v", events.Events)
	}
}
