package workflow

import (
	"context"
	"testing"
	"time"

	"flow-anything/core/flowengine"
)

func TestCompilerKeepsEngineSpecThinAndValidates(t *testing.T) {
	registry := flowengine.NewDefaultRegistry()
	compiler := NewCompiler(
		registry,
		WithNowFunc(func() time.Time { return time.Unix(100, 0).UTC() }),
		WithSnapshotIDFunc(func(WorkflowDocument, string) string { return "snapshot_test" }),
	)
	document := WorkflowDocument{
		ID: "doc_weather",
		Spec: flowengine.FlowSpec{
			ID: "flow_weather",
			Nodes: []flowengine.NodeSpec{
				{ID: "start", Type: flowengine.NodeTypeStart},
				{ID: "end", Type: flowengine.NodeTypeEnd},
			},
			Edges: []flowengine.EdgeSpec{{From: "start", To: "end"}},
		},
		UI: UIMetadata{
			Nodes: map[string]NodeUIMetadata{
				"start": {X: 10, Y: 20},
			},
		},
	}

	compiled, normalized, err := compiler.Compile(context.Background(), document)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Spec.ID != document.Spec.ID {
		t.Fatalf("compiled spec should keep document spec id")
	}
	if len(compiled.Spec.Nodes) != 2 || compiled.Spec.Nodes[0].ID != "start" {
		t.Fatalf("compiler should not rewrite node order or hidden semantics: %#v", compiled.Spec.Nodes)
	}
	if normalized.Publish.Status != PublishValidated {
		t.Fatalf("unexpected publish status: %s", normalized.Publish.Status)
	}
	if normalized.Publish.SnapshotID != "snapshot_test" || normalized.Publish.SnapshotHash == "" {
		t.Fatalf("snapshot metadata not filled: %#v", normalized.Publish)
	}
	if normalized.UI.Nodes["start"].X != 10 {
		t.Fatalf("ui metadata should be preserved")
	}
}

func TestCompilerRejectsUnknownNodeType(t *testing.T) {
	compiler := NewCompiler(flowengine.NewDefaultRegistry())
	_, _, err := compiler.Compile(context.Background(), WorkflowDocument{
		ID: "doc_invalid",
		Spec: flowengine.FlowSpec{
			ID:    "flow_invalid",
			Nodes: []flowengine.NodeSpec{{ID: "unknown", Type: "business.missing"}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
