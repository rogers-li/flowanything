package app

import (
	"context"
	"testing"

	"flow-anything/core/agentcore"
	coreconfig "flow-anything/core/config"
	"flow-anything/core/connector"
	"flow-anything/core/flowengine"
	"flow-anything/core/workflow"
)

func TestHostRunsAgentThroughCoreAgentRunner(t *testing.T) {
	ctx := context.Background()
	host, err := NewHost(baseTestBundle(), fakeModelClient{content: "hello from core agent"})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}

	result, err := host.RunAgent(ctx, AgentRequest{
		AgentID:     "agent_help",
		UserMessage: "hello",
		TraceID:     "trace_agent",
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result.Text != "hello from core agent" {
		t.Fatalf("unexpected answer: %q", result.Text)
	}
	if _, err := host.TraceStore().GetTrace(ctx, "trace_agent"); err != nil {
		t.Fatalf("expected trace from core event hooks: %v", err)
	}
}

func TestHostRunsConnectorToolThroughCoreToolsAndConnector(t *testing.T) {
	ctx := context.Background()
	bundle := baseTestBundle()
	bundle.Resources.Connectors = []coreconfig.ConnectorConfig{
		{
			ResourceMeta: coreconfig.ResourceMeta{ID: "conn_mock", Name: "Mock Connector"},
			Protocol:     coreconfig.ConnectorProtocolSpec{Kind: "mock"},
			Operations: []coreconfig.ConnectorOperationConfig{
				{
					ResourceMeta: coreconfig.ResourceMeta{ID: "op_echo", Name: "Echo Operation"},
					Request:      coreconfig.ConnectorOperationRequest{Method: "POST", Path: "/echo"},
				},
			},
		},
	}
	bundle.Resources.Tools = []coreconfig.ToolConfig{
		{
			ResourceMeta: coreconfig.ResourceMeta{ID: "tool_echo", Name: "Echo Tool"},
			Type:         coreconfig.ToolTypeConnector,
			Implementation: coreconfig.ToolImplementationSpec{
				Kind: toolImplementationConnector,
				Ref:  coreconfig.ResourceRef{Kind: coreconfig.ResourceConnectorOperation, ID: "op_echo"},
			},
		},
	}

	host, err := NewHost(
		bundle,
		fakeModelClient{content: "unused"},
		WithConnectorProtocolExecutor(connector.ProtocolFunc{
			ProtocolKind: "mock",
			ExecuteFunc: func(ctx context.Context, req connector.ProtocolRequest) (connector.ProtocolResult, error) {
				return connector.ProtocolResult{
					Output: map[string]any{"echo": req.Input["message"]},
					Raw:    req.Input,
				}, nil
			},
		}),
	)
	if err != nil {
		t.Fatalf("new host: %v", err)
	}

	result, err := host.InvokeTool(ctx, ToolRequest{
		ToolID:  "tool_echo",
		Input:   map[string]any{"message": "ping"},
		TraceID: "trace_tool",
	})
	if err != nil {
		t.Fatalf("invoke tool: %v", err)
	}
	if result.Output["echo"] != "ping" {
		t.Fatalf("unexpected tool output: %#v", result.Output)
	}
	if _, err := host.TraceStore().GetTrace(ctx, "trace_tool"); err != nil {
		t.Fatalf("expected trace from tool/connector event hooks: %v", err)
	}
}

func TestHostRegistersAndRunsSkillCapability(t *testing.T) {
	ctx := context.Background()
	bundle := baseTestBundle()
	bundle.Resources.Skills = []coreconfig.SkillConfig{
		{
			ResourceMeta: coreconfig.ResourceMeta{ID: "skill_search", Name: "Search Skill"},
			Prompt:       coreconfig.PromptConfig{System: "Search the web and summarize."},
		},
	}
	bundle.Resources.Agents[0].Reasoning = coreconfig.ReasoningConfig{Mode: "rewoo"}
	bundle.Resources.Agents[0].Skills = []coreconfig.ResourceBinding{{
		Ref: coreconfig.ResourceRef{Kind: coreconfig.ResourceSkill, ID: "skill_search"},
	}}
	model := &sequenceModelClient{contents: []string{
		`{"actions":[{"type":"skill","id":"skill_search","task":"find AI news","input":{"query":"AI news"},"reason":"needs web search"}]}`,
		"skill found AI news",
		"final answer from skill",
	}}
	host, err := NewHost(bundle, model)
	if err != nil {
		t.Fatalf("new host: %v", err)
	}

	result, err := host.RunAgent(ctx, AgentRequest{
		AgentID:     "agent_help",
		UserMessage: "find AI news",
		TraceID:     "trace_skill",
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result.Text != "final answer from skill" {
		t.Fatalf("unexpected answer: %q", result.Text)
	}
	if model.calls != 3 {
		t.Fatalf("expected parent plan, skill direct, parent final calls, got %d", model.calls)
	}
}

func TestHostReturnsAgentWorkflowTextWhenFlowOutputIsUnmapped(t *testing.T) {
	ctx := context.Background()
	bundle := baseTestBundle()
	bundle.Resources.Workflows = []coreconfig.WorkflowConfig{
		{
			ResourceMeta: coreconfig.ResourceMeta{ID: "agent_flow", Name: "Agent Workflow"},
			Spec: flowengine.FlowSpec{
				ID: "agent_flow",
				Nodes: []flowengine.NodeSpec{
					{ID: "start", Type: flowengine.NodeTypeStart, Name: "Start"},
					{
						ID:   "agent",
						Type: workflow.NodeTypeAgent,
						Name: "Writer Agent",
						Config: map[string]any{
							"agent": map[string]any{
								"id":             "agent_writer",
								"name":           "Writer Agent",
								"prompt":         "Write a response.",
								"reasoning_mode": "direct",
								"model": map[string]any{
									"provider": "mock",
									"model":    "mock-chat",
								},
							},
							"message_field": "user_request",
						},
						InputMappings: []flowengine.FieldBinding{{
							Field:   "user_request",
							Enabled: true,
							Source:  flowengine.ValueSource{Type: flowengine.SourceContext, Path: "$.flow_input.user_request"},
						}},
					},
				},
				Edges: []flowengine.EdgeSpec{{From: "start", To: "agent"}},
			},
			UI: map[string]any{"orchestration_mode": "workflow"},
		},
	}
	host, err := NewHost(bundle, fakeModelClient{content: "agent workflow answer"})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}

	result, err := host.RunWorkflow(ctx, WorkflowRequest{
		WorkflowID: "agent_flow",
		Input:      map[string]any{"user_request": "hello"},
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if result.Output["return_message"] != "agent workflow answer" {
		t.Fatalf("expected fallback return_message, got %#v", result.Output)
	}
}

func TestNormalizeAgentGraphAgentSpecUsesReWOOForLegacyReact(t *testing.T) {
	agent := normalizeAgentGraphAgentSpec(agentcore.AgentSpec{
		ID:            "agent_graph_node",
		ReasoningMode: "react",
		Policy:        agentcore.AgentPolicy{MaxIterations: 8, MaxActions: 8},
	})
	if agent.ReasoningMode != "rewoo" {
		t.Fatalf("expected agent graph node to use rewoo, got %q", agent.ReasoningMode)
	}
	if agent.Policy.MaxIterations != 1 {
		t.Fatalf("expected graph ReWOO max_iterations to be capped at 1, got %d", agent.Policy.MaxIterations)
	}
}

func baseTestBundle() coreconfig.BundleSpec {
	return coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		Kind:          coreconfig.BundleKind,
		ID:            "bundle_test",
		Name:          "Test Bundle",
		Version:       "v1",
		Runtime: coreconfig.RuntimeTargetSpec{
			Targets: []coreconfig.RuntimeTarget{coreconfig.RuntimeTest},
		},
		Resources: coreconfig.ResourceCollection{
			Models: []coreconfig.ModelConfig{
				{
					ResourceMeta: coreconfig.ResourceMeta{ID: "model_mock", Name: "Mock Model"},
					Provider:     "mock",
					Model:        "mock-chat",
				},
			},
			Agents: []coreconfig.AgentConfig{
				{
					ResourceMeta: coreconfig.ResourceMeta{ID: "agent_help", Name: "Help Agent"},
					Prompt:       coreconfig.PromptConfig{System: "You are helpful."},
					Reasoning:    coreconfig.ReasoningConfig{Mode: "direct"},
					ModelRef:     coreconfig.ResourceRef{Kind: coreconfig.ResourceModel, ID: "model_mock"},
				},
			},
		},
	}
}

type fakeModelClient struct {
	content string
}

func (m fakeModelClient) Chat(ctx agentcore.Context, req agentcore.ModelRequest) (agentcore.ModelResponse, error) {
	return agentcore.ModelResponse{
		Message: agentcore.Message{Role: "assistant", Content: m.content},
		Raw:     map[string]any{"message_count": len(req.Messages)},
		Model:   req.Model.Model,
	}, nil
}

type sequenceModelClient struct {
	calls    int
	contents []string
}

func (m *sequenceModelClient) Chat(ctx agentcore.Context, req agentcore.ModelRequest) (agentcore.ModelResponse, error) {
	content := ""
	if m.calls < len(m.contents) {
		content = m.contents[m.calls]
	}
	m.calls++
	return agentcore.ModelResponse{
		Message: agentcore.Message{Role: "assistant", Content: content},
		Raw:     map[string]any{"message_count": len(req.Messages)},
		Model:   req.Model.Model,
	}, nil
}
