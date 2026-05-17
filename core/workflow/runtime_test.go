package workflow

import (
	"context"
	"testing"

	"flow-anything/core/flowengine"
)

func TestRuntimeRunsCompiledWorkflowSpec(t *testing.T) {
	registry := flowengine.NewDefaultRegistry()
	store := flowengine.NewMemoryInstanceStore()
	engine := flowengine.NewStatefulExecutor(
		registry,
		store,
		flowengine.WithInstanceIDFunc(func() string { return "instance_runtime" }),
		flowengine.WithTokenIDFunc(sequenceID("token_runtime")),
	)
	runtime := NewRuntime(engine)
	compiled := CompiledWorkflow{
		DocumentID: "doc_simple",
		SnapshotID: "snapshot_simple",
		Spec: flowengine.FlowSpec{
			ID: "flow_simple",
			Nodes: []flowengine.NodeSpec{
				{ID: "start", Type: flowengine.NodeTypeStart},
				{ID: "end", Type: flowengine.NodeTypeEnd},
			},
			Edges: []flowengine.EdgeSpec{{From: "start", To: "end"}},
		},
	}

	instance, err := runtime.Start(context.Background(), compiled, map[string]any{"user_request": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if instance.Status != flowengine.InstanceCompleted {
		t.Fatalf("expected completed instance, got %s", instance.Status)
	}
	if instance.FlowID != "flow_simple" {
		t.Fatalf("runtime should execute compiled spec directly")
	}
}

func sequenceID(prefix string) func() string {
	next := 0
	return func() string {
		next++
		return prefix + "_" + string(rune('0'+next))
	}
}
