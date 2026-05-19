package workflow

import (
	"context"
	"testing"

	"flow-anything/core/flowengine"
	"flow-anything/core/runtimecontext"
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

type directedAgentRunner struct {
	calls []AgentRunRequest
}

func (f *directedAgentRunner) RunAgent(_ context.Context, req AgentRunRequest) (AgentRunResult, error) {
	f.calls = append(f.calls, req)
	return AgentRunResult{
		Text:   `{"text":"route to search","next_node_ids":["search","unknown"]}`,
		Output: map[string]any{"text": "route to search", "next_node_ids": []any{"search", "unknown"}},
	}, nil
}

func TestConnectorNodeOutputUsesHTTPBodyAsMappingRoot(t *testing.T) {
	output := connectorNodeOutput(ConnectorInvokeResult{
		Output: map[string]any{
			"success":     true,
			"status_code": 200,
			"body": map[string]any{
				"data": map[string]any{
					"document": map[string]any{
						"document_id": "doc_123",
					},
				},
			},
			"headers": map[string]any{"x-request-id": "req_1"},
		},
		Raw: `{"data":{"document":{"document_id":"doc_123"}}}`,
	})
	data, ok := output["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected API body data at output root, got %#v", output)
	}
	document := data["document"].(map[string]any)
	if document["document_id"] != "doc_123" {
		t.Fatalf("unexpected document id: %#v", document["document_id"])
	}
	if _, ok := output["_connector"].(map[string]any); !ok {
		t.Fatalf("expected connector envelope metadata, got %#v", output)
	}
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
	if connectors.calls[0].TraceContext.SpanID != "" || connectors.calls[0].TraceContext.ParentSpanID != runtimecontext.NodeSpanID("instance_nodes", "connector") {
		t.Fatalf("connector call should create a child span under the workflow node, got %#v", connectors.calls[0].TraceContext)
	}
	if len(tools.calls) != 1 || tools.calls[0].ToolID != "summarize_weather" {
		t.Fatalf("unexpected tool calls: %#v", tools.calls)
	}
	if tools.calls[0].TraceContext.SpanID != "" || tools.calls[0].TraceContext.ParentSpanID != runtimecontext.NodeSpanID("instance_nodes", "tool") {
		t.Fatalf("tool call should create a child span under the workflow node, got %#v", tools.calls[0].TraceContext)
	}
	if len(agents.calls) != 1 || agents.calls[0].Agent.ID != "agent_weather" {
		t.Fatalf("unexpected agent calls: %#v", agents.calls)
	}
	if agents.calls[0].TraceContext.SpanID != "" || agents.calls[0].TraceContext.ParentSpanID != runtimecontext.NodeSpanID("instance_nodes", "agent") {
		t.Fatalf("agent call should create a child span under the workflow node, got %#v", agents.calls[0].TraceContext)
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

func TestAgentNodeDirectedRoutingUsesStructuredNextNodeIDs(t *testing.T) {
	agents := &directedAgentRunner{}
	registry, err := NewDefaultWorkflowRegistry(NodeRuntime{
		Agents:     agents,
		Transforms: NewDefaultTransformRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(flowengine.NewStatefulExecutor(registry, flowengine.NewMemoryInstanceStore(), flowengine.WithInstanceIDFunc(func() string { return "instance_directed" })))
	document := WorkflowDocument{
		ID: "doc_directed",
		Spec: flowengine.FlowSpec{
			ID: "flow_directed",
			Nodes: []flowengine.NodeSpec{
				{ID: "start", Type: flowengine.NodeTypeStart},
				{
					ID:   "router",
					Type: NodeTypeAgent,
					Config: map[string]any{
						"agent": map[string]any{
							"id":             "agent_router",
							"name":           "Router",
							"reasoning_mode": "direct",
						},
						"message_field": "message",
						"metadata": map[string]any{
							"agent_routing_mode": "agent_directed",
						},
					},
					InputMappings: []flowengine.FieldBinding{{
						Field:   "message",
						Enabled: true,
						Source:  flowengine.ValueSource{Type: flowengine.SourceContext, Path: "$.flow_input.user_request"},
					}},
				},
				{ID: "search", Type: flowengine.NodeTypeNoop},
				{ID: "news", Type: flowengine.NodeTypeNoop},
			},
			Edges: []flowengine.EdgeSpec{
				{From: "start", To: "router"},
				{From: "router", To: "search"},
				{From: "router", To: "news"},
			},
		},
	}
	compiled, _, err := NewCompiler(registry).Compile(context.Background(), document)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := runtime.Start(context.Background(), compiled, map[string]any{"user_request": "AI news"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := instance.NodeStates["search"]; !ok {
		t.Fatalf("expected selected search node to run: %#v", instance.NodeStates)
	}
	if _, ok := instance.NodeStates["news"]; ok {
		t.Fatalf("agent-directed routing should skip unselected static branch")
	}
	if got := instance.NodeStates["router"].Output["text"]; got != "route to search" {
		t.Fatalf("expected structured text field to be preserved, got %#v", got)
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
			"value": map[string]any{
				"name":   "report",
				"secret": "hidden",
				"nested": map[string]any{"token": "hidden", "keep": true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	transformed := result.Output["result"].(map[string]any)
	if _, ok := transformed["secret"]; ok {
		t.Fatalf("secret field should be removed: %#v", result.Output)
	}
	nested := transformed["nested"].(map[string]any)
	if _, ok := nested["token"]; ok {
		t.Fatalf("nested token should be removed: %#v", nested)
	}
	if result.Output["removed_count"] != 2 {
		t.Fatalf("unexpected removed count: %#v", result.Output["removed_count"])
	}
}
