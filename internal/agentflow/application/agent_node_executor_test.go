package application

import (
	"context"
	"strings"
	"testing"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
)

func TestAgentNodeExecutorInvokesConfiguredAgent(t *testing.T) {
	invoker := &fakeAgentInvoker{
		result: ports.AgentInvocationResult{
			Text:    "search summary",
			TraceID: "trace_agent",
			Response: event.Response{
				TraceID: "trace_agent",
				Actions: []event.Action{
					{Type: event.ActionSpeak, Text: "search summary"},
					{Type: event.ActionEndTurn},
				},
			},
		},
	}
	executor := NewAgentNodeExecutor(invoker)

	node := domain.Node{
		ID:   "web_search_node",
		Type: domain.NodeTypeAgent,
		Name: "Web Search",
		Config: map[string]any{
			"agent_id":              "agent_web_search",
			"task":                  "search OpenAI news",
			"output_key":            "web_result",
			"runtime_system_prompt": "workflow runtime contract",
			"payload": map[string]any{
				"source": "agent_flow",
			},
		},
	}

	result, err := executor.ExecuteNode(context.Background(), ports.NodeExecutionRequest{
		Run:     testGraphRun(),
		Node:    node,
		Context: domain.NewRunContext(map[string]any{}),
	})
	if err != nil {
		t.Fatalf("ExecuteNode() error = %v", err)
	}

	if invoker.request.AgentID != "agent_web_search" {
		t.Fatalf("AgentID = %s, want agent_web_search", invoker.request.AgentID)
	}
	if invoker.request.Task != "search OpenAI news" {
		t.Fatalf("Task = %q, want configured task", invoker.request.Task)
	}
	if invoker.request.Payload["source"] != "agent_flow" {
		t.Fatalf("payload source = %v, want agent_flow", invoker.request.Payload["source"])
	}
	if invoker.request.RuntimeSystemPrompt != "workflow runtime contract" {
		t.Fatalf("RuntimeSystemPrompt = %q, want workflow runtime contract", invoker.request.RuntimeSystemPrompt)
	}
	if result.Output["text"] != "search summary" {
		t.Fatalf("Output text = %v, want search summary", result.Output["text"])
	}
	if result.Variables["web_result"] != "search summary" {
		t.Fatalf("web_result variable = %v, want search summary", result.Variables["web_result"])
	}
}

func TestBuildAgentWorkflowRuntimePromptDescribesContractsAndNextNodes(t *testing.T) {
	prompt := buildAgentWorkflowRuntimePrompt(workflow.Node{
		ID:          "writer",
		Type:        workflow.NodeTypeAgent,
		Name:        "Writer",
		Description: "Generate a draft",
		Config: map[string]any{
			"output_mode":        "json",
			"agent_routing_mode": "agent_directed",
			"write_context": map[string]any{
				"variables.draft": "$.answer",
			},
		},
	}, workflow.Graph{
		Nodes: map[id.ID]workflow.Node{
			"writer": {ID: "writer", Type: workflow.NodeTypeAgent, Name: "Writer"},
			"review": {ID: "review", Type: workflow.NodeTypeAgent, Name: "Review Agent", Description: "Review the draft"},
		},
		Edges: []workflow.Edge{{FromNodeID: "writer", ToNodeID: "review"}},
	}, map[string]any{"task": "write a summary"})

	for _, expected := range []string{
		"Input Contract",
		"task: string",
		"Output Contract",
		"answer",
		"Agent Directed Routing",
		"node_id: review",
		"next_node_ids",
		`"next_node_ids":["下游节点ID"]`,
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("runtime prompt missing %q:\n%s", expected, prompt)
		}
	}
}

func TestBuildAgentWorkflowRuntimePromptRecommendedShapeIncludesSchemaFields(t *testing.T) {
	prompt := buildAgentWorkflowRuntimePrompt(workflow.Node{
		ID:   "router",
		Type: workflow.NodeTypeAgent,
		Name: "Router",
		Config: map[string]any{
			"output_mode":        "json",
			"agent_routing_mode": "agent_directed",
			"output_schema": map[string]any{
				"type": "object",
				"x-flow-fields": []any{
					map[string]any{
						"path":        "user_request_detail",
						"type":        "string",
						"description": "清晰描述用户请求，供下游 Agent 使用",
						"required":    true,
					},
				},
			},
			"write_context": map[string]any{
				"variables.user_request_detail": "$.user_request_detail",
			},
		},
	}, workflow.Graph{
		Nodes: map[id.ID]workflow.Node{
			"router": {ID: "router", Type: workflow.NodeTypeAgent, Name: "Router"},
			"web":    {ID: "web", Type: workflow.NodeTypeAgent, Name: "Web Search"},
		},
		Edges: []workflow.Edge{{FromNodeID: "router", ToNodeID: "web"}},
	}, map[string]any{"user_request": "搜索 AI 新闻"})

	if !strings.Contains(prompt, `"user_request_detail":"请根据字段含义填写"`) {
		t.Fatalf("recommended JSON shape should include output schema fields:\n%s", prompt)
	}
}

func TestBuildAgentWorkflowRuntimePromptDoesNotForceJSONForTextWriteContext(t *testing.T) {
	prompt := buildAgentWorkflowRuntimePrompt(workflow.Node{
		ID:          "web_search",
		Type:        workflow.NodeTypeAgent,
		Name:        "Web Search Agent",
		Description: "Search the web",
		Config: map[string]any{
			"write_context": map[string]any{
				"flow_output.return_message": "$.text",
			},
		},
	}, workflow.Graph{
		Nodes: map[id.ID]workflow.Node{
			"web_search": {ID: "web_search", Type: workflow.NodeTypeAgent, Name: "Web Search Agent"},
		},
	}, map[string]any{"user_task": "search AI news"})

	if strings.Contains(prompt, "必须返回 JSON object") {
		t.Fatalf("runtime prompt should not force JSON for text write_context:\n%s", prompt)
	}
	if !strings.Contains(prompt, "输出模式为 text") {
		t.Fatalf("runtime prompt should describe text output mode:\n%s", prompt)
	}
}

func TestAgentNodeExecutorFallsBackToInputText(t *testing.T) {
	invoker := &fakeAgentInvoker{
		result: ports.AgentInvocationResult{
			Response: event.Response{
				TraceID: "trace_input",
				Actions: []event.Action{
					{Type: event.ActionDisplayText, Text: "weather answer"},
				},
			},
		},
	}
	executor := NewAgentNodeExecutor(invoker)

	_, err := executor.ExecuteNode(context.Background(), ports.NodeExecutionRequest{
		Run: testGraphRun(),
		Node: domain.Node{
			ID:   "weather_node",
			Type: domain.NodeTypeAgent,
			Name: "Weather",
			Config: map[string]any{
				"agent_id": "agent_weather",
			},
		},
		Context: domain.NewRunContext(map[string]any{"message": "上海明天天气怎么样"}),
	})
	if err != nil {
		t.Fatalf("ExecuteNode() error = %v", err)
	}
	if !strings.Contains(invoker.request.Task, "上海明天天气怎么样") || !strings.Contains(invoker.request.Task, "Current Input") {
		t.Fatalf("Task = %q, want structured input message", invoker.request.Task)
	}
}

func TestAgentNodeExecutorFallsBackToMappedUserTask(t *testing.T) {
	invoker := &fakeAgentInvoker{
		result: ports.AgentInvocationResult{
			Text: "search result",
		},
	}
	executor := NewAgentNodeExecutor(invoker)

	_, err := executor.ExecuteNode(context.Background(), ports.NodeExecutionRequest{
		Run: testGraphRun(),
		Node: domain.Node{
			ID:   "web_search_node",
			Type: domain.NodeTypeAgent,
			Name: "Web Search",
			Config: map[string]any{
				"agent_id": "agent_web_search",
				"input_schema": map[string]any{
					"type": "object",
					"x-flow-fields": []any{
						map[string]any{
							"path":        "user_task",
							"type":        "string",
							"description": "用户输入的请求，也是当前 Agent Node 需要处理的任务",
							"required":    true,
						},
					},
				},
			},
		},
		Context: domain.NewRunContext(map[string]any{"user_task": "请搜索今天的AI热门话题"}),
	})
	if err != nil {
		t.Fatalf("ExecuteNode() error = %v", err)
	}
	for _, expected := range []string{"Input Contract", "user_task", "用户输入的请求", "请搜索今天的AI热门话题"} {
		if !strings.Contains(invoker.request.Task, expected) {
			t.Fatalf("Task missing %q:\n%s", expected, invoker.request.Task)
		}
	}
}

func TestAgentNodeExecutorParsesStructuredJSONOutput(t *testing.T) {
	invoker := &fakeAgentInvoker{
		result: ports.AgentInvocationResult{
			Text: `{"summary":"AI news summary","score":0.91}`,
		},
	}
	executor := NewAgentNodeExecutor(invoker)

	result, err := executor.ExecuteNode(context.Background(), ports.NodeExecutionRequest{
		Run: testGraphRun(),
		Node: domain.Node{
			ID:   "summarizer",
			Type: domain.NodeTypeAgent,
			Config: map[string]any{
				"agent_id":    "agent_summary",
				"task":        "summarize",
				"output_mode": "json",
			},
		},
		Context: domain.NewRunContext(map[string]any{}),
	})
	if err != nil {
		t.Fatalf("ExecuteNode() error = %v", err)
	}
	if result.Output["summary"] != "AI news summary" {
		t.Fatalf("summary = %v, want parsed JSON field", result.Output["summary"])
	}
	if _, ok := result.Output["structured"].(map[string]any); !ok {
		t.Fatalf("structured output missing or invalid: %#v", result.Output["structured"])
	}
}

func TestAgentNodeExecutorParsesAgentDirectedRouting(t *testing.T) {
	invoker := &fakeAgentInvoker{
		result: ports.AgentInvocationResult{
			Text: `{"answer":"need review","next_node_ids":["review_agent"],"reason":"quality check"}`,
		},
	}
	executor := NewAgentNodeExecutor(invoker)

	result, err := executor.ExecuteNode(context.Background(), ports.NodeExecutionRequest{
		Run: testGraphRun(),
		Node: domain.Node{
			ID:   "writer",
			Type: domain.NodeTypeAgent,
			Config: map[string]any{
				"agent_id":           "agent_writer",
				"task":               "draft response",
				"agent_routing_mode": "agent_directed",
			},
		},
		Context: domain.NewRunContext(map[string]any{}),
	})
	if err != nil {
		t.Fatalf("ExecuteNode() error = %v", err)
	}
	if len(result.NextNodeIDs) != 1 || result.NextNodeIDs[0] != "review_agent" {
		t.Fatalf("NextNodeIDs = %v, want [review_agent]", result.NextNodeIDs)
	}
	decision, ok := result.Output["routing_decision"].(map[string]any)
	if !ok || decision["accepted"] != true {
		t.Fatalf("routing decision = %#v, want accepted decision", result.Output["routing_decision"])
	}
}

func TestAgentNodeExecutorRejectsMissingAgentID(t *testing.T) {
	executor := NewAgentNodeExecutor(&fakeAgentInvoker{})

	_, err := executor.ExecuteNode(context.Background(), ports.NodeExecutionRequest{
		Run:     testGraphRun(),
		Node:    domain.Node{ID: "bad", Type: domain.NodeTypeAgent},
		Context: domain.NewRunContext(map[string]any{"text": "hello"}),
	})
	if err == nil {
		t.Fatal("ExecuteNode() error = nil, want validation error")
	}
}

type fakeAgentInvoker struct {
	request ports.AgentInvocationRequest
	result  ports.AgentInvocationResult
	err     error
}

func (f *fakeAgentInvoker) InvokeAgent(ctx context.Context, request ports.AgentInvocationRequest) (ports.AgentInvocationResult, error) {
	f.request = request
	if f.err != nil {
		return ports.AgentInvocationResult{}, f.err
	}
	return f.result, nil
}

func testGraphRun() domain.FlowRun {
	return domain.FlowRun{
		ID:       id.ID("run_test"),
		TenantID: "tenant_1",
		FlowID:   "flow_test",
		Status:   domain.RunStatusRunning,
	}
}
