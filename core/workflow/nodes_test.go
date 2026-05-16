package workflow

import (
	"context"
	"testing"

	"flow-anything/core/flowengine"
)

type fakeConnectorInvoker struct {
	calls []ConnectorInvokeRequest
}

func (f *fakeConnectorInvoker) InvokeConnector(_ context.Context, req ConnectorInvokeRequest) (ConnectorInvokeResult, error) {
	f.calls = append(f.calls, req)
	return ConnectorInvokeResult{
		Output: map[string]any{"status": "ok", "city": req.Input["city"]},
		Raw:    map[string]any{"raw": true},
	}, nil
}

type fakeToolInvoker struct {
	calls []ToolInvokeRequest
}

func (f *fakeToolInvoker) InvokeTool(_ context.Context, req ToolInvokeRequest) (ToolInvokeResult, error) {
	f.calls = append(f.calls, req)
	return ToolInvokeResult{Output: map[string]any{"summary": "tool:" + req.Input["status"].(string)}}, nil
}

type fakeAgentRunner struct {
	calls []AgentRunRequest
}

func (f *fakeAgentRunner) RunAgent(_ context.Context, req AgentRunRequest) (AgentRunResult, error) {
	f.calls = append(f.calls, req)
	return AgentRunResult{
		Text:        "agent:" + req.Message,
		Output:      map[string]any{"answer": "agent:" + req.Message},
		NextNodeIDs: []string{"end"},
	}, nil
}

func TestWorkflowNodesRunConnectorToolAndAgent(t *testing.T) {
	connectors := &fakeConnectorInvoker{}
	tools := &fakeToolInvoker{}
	agents := &fakeAgentRunner{}
	registry, err := NewDefaultWorkflowRegistry(NodeRuntime{
		Connectors: connectors,
		Tools:      tools,
		Agents:     agents,
		Transforms: NewDefaultTransformRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	store := flowengine.NewMemoryInstanceStore()
	engine := flowengine.NewStatefulExecutor(
		registry,
		store,
		flowengine.WithInstanceIDFunc(func() string { return "instance_nodes" }),
		flowengine.WithTokenIDFunc(sequenceID("token_nodes")),
	)
	runtime := NewRuntime(engine)
	compiler := NewCompiler(registry)

	document := WorkflowDocument{
		ID: "doc_nodes",
		Spec: flowengine.FlowSpec{
			ID: "flow_nodes",
			Nodes: []flowengine.NodeSpec{
				{ID: "start", Type: flowengine.NodeTypeStart},
				{
					ID:   "connector",
					Type: NodeTypeConnector,
					Config: map[string]any{
						"operation_id": "weather_query",
					},
					InputMappings: []flowengine.FieldBinding{{
						Field:   "city",
						Enabled: true,
						Source:  flowengine.ValueSource{Type: flowengine.SourceContext, Path: "$.flow_input.city"},
					}},
					OutputWrites: []flowengine.ContextWrite{{
						Target:  "$.variables.weather",
						Enabled: true,
						Source:  flowengine.ValueSource{Type: flowengine.SourceNodeOutput, Path: "$"},
					}},
				},
				{
					ID:   "tool",
					Type: NodeTypeTool,
					Config: map[string]any{
						"tool_id": "summarize_weather",
					},
					InputMappings: []flowengine.FieldBinding{{
						Field:   "status",
						Enabled: true,
						Source:  flowengine.ValueSource{Type: flowengine.SourceContext, Path: "$.variables.weather.status"},
					}},
					OutputWrites: []flowengine.ContextWrite{{
						Target:  "$.variables.summary",
						Enabled: true,
						Source:  flowengine.ValueSource{Type: flowengine.SourceNodeOutput, Path: "$.summary"},
					}},
				},
				{
					ID:   "agent",
					Type: NodeTypeAgent,
					Config: map[string]any{
						"agent": map[string]any{
							"id":             "agent_weather",
							"name":           "Weather Agent",
							"reasoning_mode": "direct",
						},
						"message_field": "message",
					},
					InputMappings: []flowengine.FieldBinding{{
						Field:   "message",
						Enabled: true,
						Source:  flowengine.ValueSource{Type: flowengine.SourceContext, Path: "$.variables.summary"},
					}},
					OutputWrites: []flowengine.ContextWrite{{
						Target:  "$.flow_output.return_message",
						Enabled: true,
						Source:  flowengine.ValueSource{Type: flowengine.SourceNodeOutput, Path: "$.text"},
					}},
				},
				{ID: "skipped", Type: flowengine.NodeTypeNoop},
				{ID: "end", Type: flowengine.NodeTypeEnd},
			},
			Edges: []flowengine.EdgeSpec{
				{From: "start", To: "connector"},
				{From: "connector", To: "tool"},
				{From: "tool", To: "agent"},
				{From: "agent", To: "skipped"},
				{From: "agent", To: "end"},
			},
		},
	}

	compiled, _, err := compiler.Compile(context.Background(), document)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := runtime.Start(context.Background(), compiled, map[string]any{"city": "Shanghai"})
	if err != nil {
		t.Fatal(err)
	}
	if instance.Status != flowengine.InstanceCompleted {
		t.Fatalf("expected completed instance, got %s", instance.Status)
	}
	if len(connectors.calls) != 1 || connectors.calls[0].OperationID != "weather_query" {
		t.Fatalf("unexpected connector calls: %#v", connectors.calls)
	}
	if len(tools.calls) != 1 || tools.calls[0].ToolID != "summarize_weather" {
		t.Fatalf("unexpected tool calls: %#v", tools.calls)
	}
	if len(agents.calls) != 1 || agents.calls[0].Agent.ID != "agent_weather" {
		t.Fatalf("unexpected agent calls: %#v", agents.calls)
	}
	if _, ok := instance.NodeStates["skipped"]; ok {
		t.Fatalf("agent dynamic next node should skip static branch")
	}
	if got, _ := instance.Context.Read("$.flow_output.return_message"); got != "agent:tool:ok" {
		t.Fatalf("unexpected flow output: %#v", got)
	}
	if got, ok := instance.Context.Read("$.node_context.connector.responses.weather_query.output.status"); !ok || got != "ok" {
		t.Fatalf("connector node context not written: %#v ok=%v", got, ok)
	}
}

func TestTransformNodeRemoveFields(t *testing.T) {
	executor := NewTransformNodeExecutor(NewDefaultTransformRegistry())
	result, err := executor.Execute(context.Background(), flowengine.NodeRequest{
		Node: flowengine.NodeSpec{
			Config: map[string]any{
				"function": "json.remove_fields",
				"args": map[string]any{
					"fields": []any{"secret", "nested.token"},
				},
			},
		},
		Input: map[string]any{
			"name":   "report",
			"secret": "hidden",
			"nested": map[string]any{"token": "hidden", "keep": true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Output["secret"]; ok {
		t.Fatalf("secret field should be removed: %#v", result.Output)
	}
	nested := result.Output["nested"].(map[string]any)
	if _, ok := nested["token"]; ok {
		t.Fatalf("nested token should be removed: %#v", nested)
	}
}
