package config

import (
	"bytes"
	"strings"
	"testing"

	"flow-anything/core/flowengine"
)

func TestValidateBundleAcceptsRunnableConfigGraph(t *testing.T) {
	bundle := validBundle()
	if err := ValidateBundle(bundle); err != nil {
		t.Fatalf("expected valid bundle: %v", err)
	}

	index, err := BuildIndex(bundle)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, ok := index.Resolve(ResourceRef{Kind: ResourceConnectorOperation, ID: "connop_search"})
	if !ok {
		t.Fatal("expected connector operation to be indexed globally")
	}
	if descriptor.ParentID != "conn_search" {
		t.Fatalf("unexpected connector parent: %#v", descriptor)
	}
}

func TestValidateBundleRejectsMissingReferences(t *testing.T) {
	bundle := validBundle()
	bundle.Resources.Agents[0].Tools = append(bundle.Resources.Agents[0].Tools, ResourceBinding{
		Ref: ResourceRef{Kind: ResourceTool, ID: "tool_missing"},
	})

	err := ValidateBundle(bundle)
	if err == nil {
		t.Fatal("expected missing reference error")
	}
	if !strings.Contains(err.Error(), `referenced tool "tool_missing" does not exist`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBundleRejectsDuplicateIDs(t *testing.T) {
	bundle := validBundle()
	bundle.Resources.Tools = append(bundle.Resources.Tools, bundle.Resources.Tools[0])

	err := ValidateBundle(bundle)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
	if !strings.Contains(err.Error(), "duplicate tool id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBundleAcceptsWorkflowAgentSkillCapability(t *testing.T) {
	bundle := validBundle()
	bundle.Resources.Workflows = append(bundle.Resources.Workflows, WorkflowConfig{
		ResourceMeta: ResourceMeta{ID: "workflow_agent_graph", Name: "Agent Graph"},
		Spec: flowengine.FlowSpec{
			ID: "workflow_agent_graph",
			Nodes: []flowengine.NodeSpec{{
				ID:   "agent_search_node",
				Type: "workflow.agent",
				Config: map[string]any{
					"agent": map[string]any{
						"id":   "agent_local_search",
						"name": "Local Search Agent",
						"capabilities": []any{map[string]any{
							"id":   "skill_search",
							"type": "skill",
							"name": "Search Skill",
						}},
					},
				},
			}},
		},
	})

	if err := ValidateBundle(bundle); err != nil {
		t.Fatalf("expected workflow agent skill capability to be valid: %v", err)
	}
}

func TestValidateBundleRejectsWorkflowAgentSkillToolLeakage(t *testing.T) {
	bundle := validBundle()
	bundle.Resources.Workflows = append(bundle.Resources.Workflows, WorkflowConfig{
		ResourceMeta: ResourceMeta{ID: "workflow_leaky_agent", Name: "Leaky Agent Graph"},
		Spec: flowengine.FlowSpec{
			ID: "workflow_leaky_agent",
			Nodes: []flowengine.NodeSpec{{
				ID:   "agent_search_node",
				Type: "workflow.agent",
				Config: map[string]any{
					"agent": map[string]any{
						"id":   "agent_local_search",
						"name": "Local Search Agent",
						"capabilities": []any{
							map[string]any{"id": "skill_search", "type": "skill", "name": "Search Skill"},
							map[string]any{"id": "tool_search", "type": "tool", "name": "Search Tool"},
						},
					},
				},
			}},
		},
	})

	err := ValidateBundle(bundle)
	if err == nil {
		t.Fatal("expected workflow agent capability leakage to be rejected")
	}
	if !strings.Contains(err.Error(), `tool capability "tool_search" is owned by skill capability "skill_search"`) {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateBundleRejectsUnknownWorkflowAgentCapability(t *testing.T) {
	bundle := validBundle()
	bundle.Resources.Workflows = append(bundle.Resources.Workflows, WorkflowConfig{
		ResourceMeta: ResourceMeta{ID: "workflow_missing_capability", Name: "Missing Capability Graph"},
		Spec: flowengine.FlowSpec{
			ID: "workflow_missing_capability",
			Nodes: []flowengine.NodeSpec{{
				ID:   "agent_search_node",
				Type: "workflow.agent",
				Config: map[string]any{
					"agent": map[string]any{
						"id":   "agent_local_search",
						"name": "Local Search Agent",
						"capabilities": []any{map[string]any{
							"id":   "tool_missing",
							"type": "tool",
							"name": "Missing Tool",
						}},
					},
				},
			}},
		},
	})

	err := ValidateBundle(bundle)
	if err == nil {
		t.Fatal("expected missing workflow agent capability to be rejected")
	}
	if !strings.Contains(err.Error(), `referenced tool "tool_missing" does not exist`) {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestCheckRuntimeCompatibility(t *testing.T) {
	bundle := validBundle()
	bundle.Runtime.Targets = []RuntimeTarget{RuntimeServer, RuntimeMobile}
	bundle.Resources.Tools[0].Runtime.Network = true
	bundle.Resources.Tools[0].Runtime.Capabilities = []CapabilityRequirement{{
		Name:     "http.client",
		Required: true,
	}}

	err := CheckRuntimeCompatibility(bundle, RuntimeManifest{
		RuntimeID: "mobile_runtime",
		Target:    RuntimeIOS,
		Version:   "v1",
		Capabilities: []CapabilitySupport{
			{Name: "network"},
			{Name: "http.client"},
		},
	})
	if err != nil {
		t.Fatalf("expected compatible runtime: %v", err)
	}

	err = CheckRuntimeCompatibility(bundle, RuntimeManifest{
		RuntimeID: "offline_runtime",
		Target:    RuntimeDesktop,
		Version:   "v1",
	})
	if err == nil {
		t.Fatal("expected compatibility error")
	}
	if !strings.Contains(err.Error(), "not allowed") || !strings.Contains(err.Error(), "network") {
		t.Fatalf("unexpected compatibility error: %v", err)
	}
}

func TestBundleJSONRoundTripUsesConfigProtocolFields(t *testing.T) {
	bundle := validBundle()
	var buffer bytes.Buffer
	if err := WriteBundleJSON(&buffer, bundle); err != nil {
		t.Fatal(err)
	}
	encoded := buffer.String()
	if !strings.Contains(encoded, `"schema_version"`) || !strings.Contains(encoded, `"input_mappings"`) {
		t.Fatalf("encoded bundle should use config protocol field names: %s", encoded)
	}

	loaded, err := LoadBundleJSON(strings.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateBundle(loaded); err != nil {
		t.Fatalf("round-tripped bundle should remain valid: %v", err)
	}
}

func validBundle() BundleSpec {
	return BundleSpec{
		SchemaVersion: SchemaVersionV1,
		Kind:          BundleKind,
		ID:            "bundle_search",
		Name:          "Search Assistant Bundle",
		Version:       "2026.05.16-001",
		Resources: ResourceCollection{
			Models: []ModelConfig{{
				ResourceMeta: ResourceMeta{ID: "model_deepseek", Name: "DeepSeek"},
				Provider:     "deepseek",
				Model:        "deepseek-v4-flash",
			}},
			Connectors: []ConnectorConfig{{
				ResourceMeta: ResourceMeta{ID: "conn_search", Name: "Search API"},
				Protocol:     ConnectorProtocolSpec{Kind: "http", BaseURL: "https://search.example.com"},
				Auth:         ConnectorAuthSpec{Type: "api_key", SecretRef: "SEARCH_API_KEY"},
				Operations: []ConnectorOperationConfig{{
					ResourceMeta: ResourceMeta{ID: "connop_search", Name: "Search"},
					Request:      ConnectorOperationRequest{Method: "POST", Path: "/search"},
					InputSchema:  []SchemaField{{Name: "query", Type: "string", Required: true}},
					OutputSchema: []SchemaField{{Name: "results", Type: "array"}},
				}},
			}},
			Tools: []ToolConfig{{
				ResourceMeta: ResourceMeta{ID: "tool_search", Name: "Search Tool"},
				Type:         ToolTypeConnector,
				InputSchema:  []SchemaField{{Name: "query", Type: "string", Required: true}},
				OutputSchema: []SchemaField{{Name: "results", Type: "array"}},
				Implementation: ToolImplementationSpec{
					Kind: "connector",
					Ref:  ResourceRef{Kind: ResourceConnectorOperation, ID: "connop_search"},
				},
				Policy: ExecutionPolicy{Timeout: "30s"},
			}},
			Skills: []SkillConfig{{
				ResourceMeta: ResourceMeta{ID: "skill_search", Name: "Search Skill"},
				Prompt:       PromptConfig{System: "Use search tools to answer user questions."},
				Tools: []ResourceBinding{{
					Ref: ResourceRef{Kind: ResourceTool, ID: "tool_search"},
				}},
			}},
			Workflows: []WorkflowConfig{{
				ResourceMeta: ResourceMeta{ID: "workflow_report", Name: "Report Workflow"},
				Spec: flowengine.FlowSpec{
					ID: "workflow_report",
					Nodes: []flowengine.NodeSpec{{
						ID:   "start",
						Type: flowengine.NodeTypeStart,
						InputMappings: []flowengine.FieldBinding{{
							Field:   "user_request",
							Enabled: true,
							Source:  flowengine.ValueSource{Type: flowengine.SourceContext, Path: "$.flow_input.user_request"},
						}},
					}},
				},
			}},
			Agents: []AgentConfig{{
				ResourceMeta: ResourceMeta{ID: "agent_search", Name: "Search Agent"},
				Prompt:       PromptConfig{System: "You are a search assistant."},
				Reasoning:    ReasoningConfig{Mode: "action-planning"},
				ModelRef:     ResourceRef{Kind: ResourceModel, ID: "model_deepseek"},
				Skills: []ResourceBinding{{
					Ref: ResourceRef{Kind: ResourceSkill, ID: "skill_search"},
				}},
				Tools: []ResourceBinding{{
					Ref: ResourceRef{Kind: ResourceTool, ID: "tool_search"},
				}},
				Workflows: []ResourceBinding{{
					Ref: ResourceRef{Kind: ResourceWorkflow, ID: "workflow_report"},
				}},
			}},
		},
	}
}
