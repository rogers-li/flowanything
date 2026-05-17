package editor

import (
	"strings"
	"testing"

	"flow-anything/core/config"
	"flow-anything/core/flowengine"
)

func TestInspectDraftAddsCanvasDiagnostics(t *testing.T) {
	bundle := editorTestBundle()
	bundle.Resources.Workflows[0].Spec.Nodes = append(bundle.Resources.Workflows[0].Spec.Nodes, flowengine.NodeSpec{
		ID:   "start_2",
		Type: flowengine.NodeTypeStart,
	})

	inspection := InspectDraft(bundle)
	if inspection.Publishable {
		t.Fatal("expected draft with two start nodes to be non-publishable")
	}
	if !containsDiagnostic(inspection.Diagnostics, "multiple start nodes") {
		t.Fatalf("expected multiple start diagnostic, got %#v", inspection.Diagnostics)
	}
}

func TestListBindableResourcesFiltersDisabledResources(t *testing.T) {
	bundle := editorTestBundle()
	bundle.Resources.Tools = []config.ToolConfig{
		{
			ResourceMeta: config.ResourceMeta{ID: "tool_live", Name: "Live Tool"},
			Type:         config.ToolTypeNative,
			Implementation: config.ToolImplementationSpec{
				Kind: "native",
			},
		},
		{
			ResourceMeta: config.ResourceMeta{ID: "tool_disabled", Name: "Disabled Tool", Disabled: true},
			Type:         config.ToolTypeNative,
			Implementation: config.ToolImplementationSpec{
				Kind: "native",
			},
		},
	}

	resources, err := ListBindableResources(bundle, BindingFilter{Kinds: []config.ResourceKind{config.ResourceTool}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].ID != "tool_live" {
		t.Fatalf("unexpected bindable resources: %#v", resources)
	}
}

func TestApplyPatchAndDiffBundles(t *testing.T) {
	bundle := editorTestBundle()
	edited, err := ApplyPatch(bundle, PatchSet{Operations: []PatchOperation{{
		Op:    PatchReplace,
		Path:  "/resources/agents/0/prompt/system",
		Value: "You are a better assistant.",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Resources.Agents[0].Prompt.System != "You are a better assistant." {
		t.Fatalf("patch did not update prompt: %#v", edited.Resources.Agents[0].Prompt)
	}

	patch, err := DiffBundles(bundle, edited)
	if err != nil {
		t.Fatal(err)
	}
	if len(patch.Operations) == 0 {
		t.Fatal("expected diff patch")
	}
	roundTripped, err := ApplyPatch(bundle, patch)
	if err != nil {
		t.Fatal(err)
	}
	if roundTripped.Resources.Agents[0].Prompt.System != edited.Resources.Agents[0].Prompt.System {
		t.Fatalf("diff patch did not reproduce edit: %#v", patch)
	}
}

func TestWorkflowEditHelpersReturnEditedDraft(t *testing.T) {
	bundle := editorTestBundle()
	updated, err := UpsertWorkflowNode(bundle, "workflow_main", flowengine.NodeSpec{
		ID:   "agent_1",
		Type: "agent",
		Name: "Agent Node",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Resources.Workflows[0].Spec.Nodes) != 1 {
		t.Fatal("workflow edit mutated original bundle")
	}
	if len(updated.Resources.Workflows[0].Spec.Nodes) != 2 {
		t.Fatalf("expected node to be appended: %#v", updated.Resources.Workflows[0].Spec.Nodes)
	}

	updated, err = UpsertWorkflowEdge(updated, "workflow_main", flowengine.EdgeSpec{From: "start", To: "agent_1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Resources.Workflows[0].Spec.Edges) != 1 {
		t.Fatalf("expected edge to be appended: %#v", updated.Resources.Workflows[0].Spec.Edges)
	}

	updated, err = DeleteWorkflowNode(updated, "workflow_main", "agent_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Resources.Workflows[0].Spec.Nodes) != 1 || len(updated.Resources.Workflows[0].Spec.Edges) != 0 {
		t.Fatalf("expected node and connected edge to be removed: %#v", updated.Resources.Workflows[0].Spec)
	}
}

func TestMigrateBundleNormalizesV1(t *testing.T) {
	bundle := editorTestBundle()
	bundle.Kind = ""

	migrated, patch, err := MigrateBundle(bundle, config.SchemaVersionV1)
	if err != nil {
		t.Fatal(err)
	}
	if migrated.Kind != config.BundleKind {
		t.Fatalf("expected bundle kind to be normalized: %#v", migrated.Kind)
	}
	if len(patch.Operations) == 0 {
		t.Fatal("expected migration patch to show normalized kind")
	}
}

func containsDiagnostic(diagnostics []config.Diagnostic, text string) bool {
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, text) {
			return true
		}
	}
	return false
}

func editorTestBundle() config.BundleSpec {
	return config.BundleSpec{
		SchemaVersion: config.SchemaVersionV1,
		Kind:          config.BundleKind,
		ID:            "bundle_editor",
		Name:          "Editor Test Bundle",
		Version:       "v1",
		Resources: config.ResourceCollection{
			Models: []config.ModelConfig{{
				ResourceMeta: config.ResourceMeta{ID: "model_default", Name: "Default Model"},
				Provider:     "mock",
				Model:        "mock-model",
			}},
			Agents: []config.AgentConfig{{
				ResourceMeta: config.ResourceMeta{ID: "agent_default", Name: "Default Agent"},
				Prompt:       config.PromptConfig{System: "You are a helpful assistant."},
				ModelRef:     config.ResourceRef{Kind: config.ResourceModel, ID: "model_default"},
			}},
			Workflows: []config.WorkflowConfig{{
				ResourceMeta: config.ResourceMeta{ID: "workflow_main", Name: "Main Workflow"},
				Spec: flowengine.FlowSpec{
					ID:      "workflow_main",
					Name:    "Main Workflow",
					Version: "v1",
					Nodes: []flowengine.NodeSpec{{
						ID:   "start",
						Type: flowengine.NodeTypeStart,
						Name: "Start",
					}},
				},
				UI: map[string]any{
					"nodes": map[string]any{
						"start": map[string]any{"x": 10, "y": 20},
					},
				},
			}},
		},
	}
}
