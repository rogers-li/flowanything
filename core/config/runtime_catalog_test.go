package config

import (
	"testing"

	"flow-anything/core/connector"
	"flow-anything/core/tools"
)

func TestInspectBundleReturnsEditorState(t *testing.T) {
	bundle := validBundle()
	state := InspectBundle(bundle)
	if len(state.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics: %#v", state.Diagnostics)
	}
	if len(state.Resources) == 0 {
		t.Fatal("expected editor resources")
	}
	if !hasDependency(state.Dependencies, ResourceAgent, "agent_search", ResourceTool, "tool_search") {
		t.Fatalf("expected agent -> tool dependency: %#v", state.Dependencies)
	}

	bundle.Resources.Agents[0].Tools = append(bundle.Resources.Agents[0].Tools, ResourceBinding{
		Ref: ResourceRef{Kind: ResourceTool, ID: "missing_tool"},
	})
	state = InspectBundle(bundle)
	if len(state.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for invalid bundle")
	}
}

func TestCompileRuntimeCatalogConvertsConfigToCoreSpecs(t *testing.T) {
	bundle := validBundle()
	bundle.Resources.Models[0].DefaultParameters = map[string]any{
		"temperature": 0.2,
		"max_tokens":  1024,
	}
	catalog, err := CompileRuntimeCatalog(bundle)
	if err != nil {
		t.Fatal(err)
	}

	agent := catalog.Agents["agent_search"]
	if agent.ID != "agent_search" || agent.Model.Provider != "deepseek" || agent.Model.MaxTokens != 1024 {
		t.Fatalf("unexpected agent spec: %#v", agent)
	}
	if len(agent.Capabilities) != 3 {
		t.Fatalf("expected skill/tool/workflow capabilities, got %#v", agent.Capabilities)
	}

	tool := catalog.Tools["tool_search"]
	if tool.Type != tools.ToolTypeConnector || tool.Implementation.Ref != "connop_search" || tool.Policy.Timeout == 0 {
		t.Fatalf("unexpected tool spec: %#v", tool)
	}

	operation := catalog.ConnectorOperations["connop_search"]
	if operation.ConnectorID != "conn_search" || operation.Request.Method != "POST" {
		t.Fatalf("unexpected operation spec: %#v", operation)
	}
	if _, ok := any(operation).(connector.OperationSpec); !ok {
		t.Fatal("operation should be connector.OperationSpec")
	}
}

func hasDependency(edges []DependencyEdge, fromKind ResourceKind, fromID string, toKind ResourceKind, toID string) bool {
	for _, edge := range edges {
		if edge.From.Kind == fromKind && edge.From.ID == fromID && edge.To.Kind == toKind && edge.To.ID == toID {
			return true
		}
	}
	return false
}
