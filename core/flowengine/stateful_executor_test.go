package flowengine

import (
	"context"
	"testing"
)

func TestStatefulExecutorWaitsAndResumes(t *testing.T) {
	registry := NewDefaultRegistry()
	if err := registry.Register(FuncNodeExecutor{
		NodeType: "business.answer",
		Fn: func(_ context.Context, req NodeRequest) (NodeResult, error) {
			approved := req.Input["approved"]
			return NodeResult{Output: map[string]any{"message": "approved=" + boolText(approved)}}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryInstanceStore()
	events := &MemoryEventSink{}
	executor := NewStatefulExecutor(
		registry,
		store,
		WithStatefulEventSink(events),
		WithInstanceIDFunc(func() string { return "instance_wait" }),
		WithTokenIDFunc(sequenceID("token_wait")),
	)

	spec := FlowSpec{
		ID: "flow_wait",
		Nodes: []NodeSpec{
			{ID: "start", Type: NodeTypeStart},
			{
				ID:   "wait_approval",
				Type: NodeTypeWait,
				Config: map[string]any{
					"event_type": "approval.completed",
					"event_key_source": map[string]any{
						"type": SourceContext,
						"path": "$.flow_input.approval_id",
					},
				},
				OutputWrites: []ContextWrite{{
					Target:  "$.variables.approval",
					Enabled: true,
					Source:  ValueSource{Type: SourceNodeOutput, Path: "$.payload"},
				}},
			},
			{
				ID:   "answer",
				Type: "business.answer",
				InputMappings: []FieldBinding{{
					Field:   "approved",
					Enabled: true,
					Source:  ValueSource{Type: SourceContext, Path: "$.variables.approval.approved"},
				}},
				OutputWrites: []ContextWrite{{
					Target:  "$.flow_output.return_message",
					Enabled: true,
					Source:  ValueSource{Type: SourceNodeOutput, Path: "$.message"},
				}},
			},
		},
		Edges: []EdgeSpec{
			{From: "start", To: "wait_approval"},
			{From: "wait_approval", To: "answer"},
		},
	}

	waiting, err := executor.Start(context.Background(), spec, map[string]any{"approval_id": "approval_123"})
	if err != nil {
		t.Fatal(err)
	}
	if waiting.Status != InstanceWaiting {
		t.Fatalf("expected waiting instance, got %s", waiting.Status)
	}
	if waiting.NodeStates["wait_approval"].Status != NodeWaiting {
		t.Fatalf("expected wait node status, got %#v", waiting.NodeStates["wait_approval"])
	}
	if !hasFlowEvent(events.Events, EventFlowWaiting) {
		t.Fatalf("expected flow waiting event: %#v", events.Events)
	}

	completed, err := executor.Resume(context.Background(), spec, waiting.InstanceID, ExternalEvent{
		Type:    "approval.completed",
		Key:     "approval_123",
		Payload: map[string]any{"approved": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != InstanceCompleted {
		t.Fatalf("expected completed instance, got %s", completed.Status)
	}
	if got, _ := completed.Context.Read("$.flow_output.return_message"); got != "approved=true" {
		t.Fatalf("unexpected flow output: %#v", got)
	}
}

func TestStatefulExecutorParallelJoinWaitsForAllBranches(t *testing.T) {
	registry := NewDefaultRegistry()
	if err := registry.Register(FuncNodeExecutor{
		NodeType: "business.branch",
		Fn: func(_ context.Context, req NodeRequest) (NodeResult, error) {
			return NodeResult{Output: map[string]any{"branch": req.Node.ID}}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryInstanceStore()
	executor := NewStatefulExecutor(
		registry,
		store,
		WithInstanceIDFunc(func() string { return "instance_join" }),
		WithTokenIDFunc(sequenceID("token_join")),
	)

	completed, err := executor.Start(context.Background(), FlowSpec{
		ID: "flow_join",
		Nodes: []NodeSpec{
			{ID: "start", Type: NodeTypeStart},
			{ID: "fork", Type: NodeTypeParallel},
			{ID: "branch_a", Type: "business.branch"},
			{ID: "branch_b", Type: "business.branch"},
			{ID: "join", Type: NodeTypeJoin},
			{ID: "end", Type: NodeTypeEnd},
		},
		Edges: []EdgeSpec{
			{From: "start", To: "fork"},
			{From: "fork", To: "branch_a"},
			{From: "fork", To: "branch_b"},
			{From: "branch_a", To: "join"},
			{From: "branch_b", To: "join"},
			{From: "join", To: "end"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != InstanceCompleted {
		t.Fatalf("expected completed instance, got %s", completed.Status)
	}
	joinState := completed.JoinStates["join"]
	if !joinState.Completed {
		t.Fatalf("expected completed join state: %#v", joinState)
	}
	if !joinState.ArrivedNodes["branch_a"] || !joinState.ArrivedNodes["branch_b"] {
		t.Fatalf("expected both branches arrived: %#v", joinState.ArrivedNodes)
	}
	if completed.NodeStates["end"].Status != NodeCompleted {
		t.Fatalf("expected end node completed: %#v", completed.NodeStates["end"])
	}
}

func boolText(value any) string {
	if value == true {
		return "true"
	}
	return "false"
}

func sequenceID(prefix string) func() string {
	next := 0
	return func() string {
		next++
		return prefix + "_" + string(rune('0'+next))
	}
}

func hasFlowEvent(events []FlowEvent, eventType FlowEventType) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}
