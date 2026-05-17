package flowengine

import (
	"context"
	"reflect"
	"testing"

	"flow-anything/core/runtimecontext"
)

func TestExecutorRunsDAGAndWritesContext(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(FuncNodeExecutor{
		NodeType: "transform",
		Fn: func(_ context.Context, req NodeRequest) (NodeResult, error) {
			query := req.Input["query"].(string)
			return NodeResult{Output: map[string]any{"normalized": "normalized:" + query}}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(FuncNodeExecutor{
		NodeType: "answer",
		Fn: func(_ context.Context, req NodeRequest) (NodeResult, error) {
			query := req.Input["normalized_query"].(string)
			return NodeResult{Output: map[string]any{"text": "answer for " + query}}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	events := &MemoryEventSink{}
	executor := NewExecutor(registry, WithEventSink(events), WithRunIDFunc(func() string { return "run_test" }))
	result, err := executor.Execute(context.Background(), FlowSpec{
		ID: "flow_search",
		Nodes: []NodeSpec{
			{
				ID:   "normalize",
				Type: "transform",
				Name: "Normalize Query",
				InputMappings: []FieldBinding{{
					Field:   "query",
					Enabled: true,
					Source:  ValueSource{Type: SourceContext, Path: "$.flow_input.query"},
				}},
				OutputWrites: []ContextWrite{{
					Target:  "$.variables.normalized_query",
					Enabled: true,
					Source:  ValueSource{Type: SourceNodeOutput, Path: "$.normalized"},
				}},
			},
			{
				ID:   "answer",
				Type: "answer",
				Name: "Answer",
				InputMappings: []FieldBinding{{
					Field:   "normalized_query",
					Enabled: true,
					Source:  ValueSource{Type: SourceContext, Path: "$.variables.normalized_query"},
				}},
				OutputWrites: []ContextWrite{{
					Target:  "$.flow_output.return_message",
					Enabled: true,
					Source:  ValueSource{Type: SourceNodeOutput, Path: "$.text"},
				}},
			},
		},
		Edges: []EdgeSpec{{From: "normalize", To: "answer"}},
	}, map[string]any{"query": "today ai news"})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result.NodeOrder, []string{"normalize", "answer"}) {
		t.Fatalf("unexpected node order: %#v", result.NodeOrder)
	}
	if got, _ := result.Context.Read("$.flow_output.return_message"); got != "answer for normalized:today ai news" {
		t.Fatalf("unexpected flow output: %#v", got)
	}
	if len(events.Events) != 9 {
		t.Fatalf("expected 9 flow events, got %d", len(events.Events))
	}
	if events.Events[0].Type != EventFlowStarted {
		t.Fatalf("first event should be flow started, got %s", events.Events[0].Type)
	}
	if events.Events[len(events.Events)-1].Type != EventFlowCompleted {
		t.Fatalf("last event should be flow completed, got %s", events.Events[len(events.Events)-1].Type)
	}
}

func TestExecutorHonorsDynamicNextNodeIDs(t *testing.T) {
	registry := NewRegistry()
	_ = registry.Register(FuncNodeExecutor{
		NodeType: "router",
		Fn: func(context.Context, NodeRequest) (NodeResult, error) {
			return NodeResult{NextNodeIDs: []string{"selected"}}, nil
		},
	})
	_ = registry.Register(FuncNodeExecutor{
		NodeType: "leaf",
		Fn: func(_ context.Context, req NodeRequest) (NodeResult, error) {
			return NodeResult{Output: map[string]any{"node": req.Node.ID}}, nil
		},
	})

	executor := NewExecutor(registry, WithRunIDFunc(func() string { return "run_route" }))
	result, err := executor.Execute(context.Background(), FlowSpec{
		ID: "flow_route",
		Nodes: []NodeSpec{
			{ID: "router", Type: "router"},
			{ID: "selected", Type: "leaf"},
			{ID: "skipped", Type: "leaf"},
		},
		Edges: []EdgeSpec{
			{From: "router", To: "selected"},
			{From: "router", To: "skipped"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.NodeOrder, []string{"router", "selected"}) {
		t.Fatalf("unexpected node order: %#v", result.NodeOrder)
	}
}

func TestExecutorPublishesFailureEvents(t *testing.T) {
	registry := NewRegistry()
	_ = registry.Register(FuncNodeExecutor{
		NodeType: "broken",
		Fn: func(context.Context, NodeRequest) (NodeResult, error) {
			return NodeResult{}, assertErr("boom")
		},
	})

	events := &MemoryEventSink{}
	executor := NewExecutor(registry, WithEventSink(events), WithRunIDFunc(func() string { return "run_fail" }))
	_, err := executor.Execute(context.Background(), FlowSpec{
		ID:    "flow_fail",
		Nodes: []NodeSpec{{ID: "broken", Type: "broken"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !hasEvent(events.Events, EventNodeFailed) {
		t.Fatalf("expected node failed event: %#v", events.Events)
	}
	if !hasEvent(events.Events, EventFlowFailed) {
		t.Fatalf("expected flow failed event: %#v", events.Events)
	}
}

func TestExecutorPropagatesTraceContextToNodeAndEvents(t *testing.T) {
	registry := NewRegistry()
	var nodeTrace runtimecontext.TraceContext
	_ = registry.Register(FuncNodeExecutor{
		NodeType: "capture",
		Fn: func(ctx context.Context, req NodeRequest) (NodeResult, error) {
			var ok bool
			nodeTrace, ok = runtimecontext.TraceContextFrom(ctx)
			if !ok {
				t.Fatal("expected node context to carry trace context")
			}
			return NodeResult{Output: map[string]any{"ok": true}}, nil
		},
	})
	events := &MemoryEventSink{}
	executor := NewExecutor(registry, WithEventSink(events), WithRunIDFunc(func() string { return "run_trace" }))
	_, err := executor.Execute(context.Background(), FlowSpec{
		ID:    "flow_trace",
		Nodes: []NodeSpec{{ID: "capture", Type: "capture"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if nodeTrace.TraceID != "run_trace" || nodeTrace.SpanID != runtimecontext.NodeSpanID("run_trace", "capture") {
		t.Fatalf("unexpected node trace context: %#v", nodeTrace)
	}
	if nodeTrace.ParentSpanID != runtimecontext.FlowSpanID("run_trace") {
		t.Fatalf("node parent should be flow span: %#v", nodeTrace)
	}
	if events.Events[0].TraceContext.SpanID != runtimecontext.FlowSpanID("run_trace") {
		t.Fatalf("flow event should carry flow span: %#v", events.Events[0].TraceContext)
	}
	if !hasEventTrace(events.Events, runtimecontext.NodeSpanID("run_trace", "capture")) {
		t.Fatalf("expected node events to carry node span: %#v", events.Events)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func hasEvent(events []FlowEvent, eventType FlowEventType) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func hasEventTrace(events []FlowEvent, spanID string) bool {
	for _, event := range events {
		if event.TraceContext.SpanID == spanID {
			return true
		}
	}
	return false
}
