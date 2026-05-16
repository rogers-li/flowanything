package flowengine

import (
	"context"
	"reflect"
	"testing"
)

func TestConditionNodeRoutesByContext(t *testing.T) {
	registry := NewDefaultRegistry()
	_ = registry.Register(FuncNodeExecutor{
		NodeType: "business.leaf",
		Fn: func(_ context.Context, req NodeRequest) (NodeResult, error) {
			return NodeResult{Output: map[string]any{"leaf": req.Node.ID}}, nil
		},
	})

	executor := NewExecutor(registry, WithRunIDFunc(func() string { return "run_condition" }))
	result, err := executor.Execute(context.Background(), FlowSpec{
		ID: "flow_condition",
		Nodes: []NodeSpec{
			{ID: "start", Type: NodeTypeStart},
			{
				ID:   "route",
				Type: NodeTypeCondition,
				Config: map[string]any{
					"branches": []any{
						map[string]any{
							"name":          "weather",
							"next_node_ids": []string{"weather"},
							"rules": []any{
								map[string]any{
									"left":     map[string]any{"type": SourceContext, "path": "$.flow_input.intent"},
									"operator": ConditionOpEquals,
									"right":    map[string]any{"type": SourceConst, "value": "weather"},
								},
							},
							"context_writes": []ContextWrite{{
								Target:  "$.variables.selected_branch",
								Enabled: true,
								Source:  ValueSource{Type: SourceNodeOutput, Path: "$.branch"},
							}},
						},
						map[string]any{
							"name":          "search",
							"next_node_ids": []string{"search"},
							"rules": []any{
								map[string]any{
									"left":     map[string]any{"type": SourceContext, "path": "$.flow_input.intent"},
									"operator": ConditionOpEquals,
									"right":    map[string]any{"type": SourceConst, "value": "search"},
								},
							},
						},
					},
				},
			},
			{ID: "weather", Type: "business.leaf"},
			{ID: "search", Type: "business.leaf"},
		},
		Edges: []EdgeSpec{
			{From: "start", To: "route"},
			{From: "route", To: "weather"},
			{From: "route", To: "search"},
		},
	}, map[string]any{"intent": "weather"})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result.NodeOrder, []string{"start", "route", "weather"}) {
		t.Fatalf("unexpected node order: %#v", result.NodeOrder)
	}
	if selected, _ := result.Context.Read("$.variables.selected_branch"); selected != "weather" {
		t.Fatalf("unexpected selected branch: %#v", selected)
	}
}

func TestConditionNodeCanStopFlowWithEmptyNextNodeIDs(t *testing.T) {
	registry := NewDefaultRegistry()
	_ = registry.Register(FuncNodeExecutor{
		NodeType: "business.unexpected",
		Fn: func(context.Context, NodeRequest) (NodeResult, error) {
			t.Fatal("unexpected downstream node execution")
			return NodeResult{}, nil
		},
	})

	executor := NewExecutor(registry, WithRunIDFunc(func() string { return "run_stop" }))
	result, err := executor.Execute(context.Background(), FlowSpec{
		ID: "flow_stop",
		Nodes: []NodeSpec{
			{
				ID:   "route",
				Type: NodeTypeCondition,
				Config: map[string]any{
					"branches": []any{
						map[string]any{
							"name":          "end",
							"next_node_ids": []string{},
							"rules": []any{
								map[string]any{
									"left":     map[string]any{"type": SourceContext, "path": "$.flow_input.should_stop"},
									"operator": ConditionOpEquals,
									"right":    map[string]any{"type": SourceConst, "value": true},
								},
							},
						},
					},
				},
			},
			{ID: "unexpected", Type: "business.unexpected"},
		},
		Edges: []EdgeSpec{{From: "route", To: "unexpected"}},
	}, map[string]any{"should_stop": true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.NodeOrder, []string{"route"}) {
		t.Fatalf("unexpected node order: %#v", result.NodeOrder)
	}
}

func TestEndNodeStopsFlow(t *testing.T) {
	registry := NewDefaultRegistry()
	_ = registry.Register(FuncNodeExecutor{
		NodeType: "business.unexpected",
		Fn: func(context.Context, NodeRequest) (NodeResult, error) {
			t.Fatal("unexpected downstream node execution")
			return NodeResult{}, nil
		},
	})

	executor := NewExecutor(registry, WithRunIDFunc(func() string { return "run_end" }))
	result, err := executor.Execute(context.Background(), FlowSpec{
		ID: "flow_end",
		Nodes: []NodeSpec{
			{ID: "end", Type: NodeTypeEnd},
			{ID: "unexpected", Type: "business.unexpected"},
		},
		Edges: []EdgeSpec{{From: "end", To: "unexpected"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.NodeOrder, []string{"end"}) {
		t.Fatalf("unexpected node order: %#v", result.NodeOrder)
	}
}
