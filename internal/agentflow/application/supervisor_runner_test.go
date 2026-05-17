package application

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/infrastructure"
	"flow-anything/internal/agentflow/ports"
	orchestration "flow-anything/internal/agentorchestration"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestSupervisorRunnerPlansCallsSubAgentAndSynthesizes(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	catalog := fakeSupervisorAgentCatalog{
		"agent_supervisor": {
			ID:           "agent_supervisor",
			TenantID:     "tenant_1",
			Name:         "Supervisor",
			Status:       agent.StatusEnabled,
			SystemPrompt: "Route weather requests to Weather Agent, then verify and summarize the result.",
		},
		"agent_weather": {ID: "agent_weather", TenantID: "tenant_1", Name: "Weather Agent", Description: "Answer weather questions.", Status: agent.StatusEnabled},
	}
	invoker := &sequenceAgentInvoker{
		results: []ports.AgentInvocationResult{
			responseWithText(`{"actions":[{"type":"agent","agent_id":"agent_weather","task":"查询上海明天天气","reason":"weather question"}]}`),
			responseWithText(`{"actions":[],"final_answer_if_no_action":"上海明天多云，22 到 28 度。"}`),
			responseWithText("上海明天多云，气温约 22 到 28 度。"),
		},
	}
	runner := NewSupervisorRunner(slog.Default(), store, invoker, catalog, nil)

	run, err := runner.Execute(context.Background(), agentflow.Spec{
		ID:                "flow_weather_supervisor",
		TenantID:          tenant.ID("tenant_1"),
		Name:              "Weather Supervisor",
		Status:            agentflow.StatusEnabled,
		OrchestrationMode: agentflow.OrchestrationModeSupervisor,
		Supervisor: agentflow.SupervisorSpec{
			SupervisorAgentID: "agent_supervisor",
			SubAgentIDs:       []id.ID{"agent_weather"},
			MaxDepth:          1,
			MaxSubAgentCalls:  3,
		},
		Graph: domain.FlowGraph{
			ID:          "flow_weather_supervisor",
			TenantID:    "tenant_1",
			Name:        "Weather Supervisor",
			Status:      domain.FlowStatusEnabled,
			EntryNodeID: "start",
			Nodes: map[id.ID]domain.Node{
				"start": {
					ID:   "start",
					Type: domain.NodeTypeStart,
					Name: "Start",
				},
				"supervisor": {
					ID:   "supervisor",
					Type: domain.NodeTypeSupervisor,
					Name: "Supervisor",
					Config: map[string]any{
						"agent_id":   "agent_supervisor",
						"agent_mode": "local",
					},
				},
				"weather": {
					ID:   "weather",
					Type: domain.NodeTypeAgent,
					Name: "Weather Agent",
					Config: map[string]any{
						"agent_id":   "agent_weather",
						"agent_mode": "existing",
					},
				},
			},
			Edges: []domain.Edge{
				{ID: "start-supervisor", FromNodeID: "start", ToNodeID: "supervisor", Type: domain.EdgeTypeDefault},
				{ID: "supervisor-weather", FromNodeID: "supervisor", ToNodeID: "weather", Type: domain.EdgeTypeDefault},
			},
		},
		Version: "v1",
	}, map[string]any{"message": "上海明天天气怎么样"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run status = %s, want succeeded", run.Status)
	}
	if run.Output["text"] != "上海明天多云，气温约 22 到 28 度。" {
		t.Fatalf("final text = %v", run.Output["text"])
	}
	if len(invoker.requests) != 3 {
		t.Fatalf("agent invocations = %d, want 3", len(invoker.requests))
	}
	if !strings.Contains(invoker.requests[0].Task, "上海明天天气怎么样") {
		t.Fatalf("planning task should include raw user request, got %q", invoker.requests[0].Task)
	}
	if invoker.requests[1].AgentID != "agent_weather" {
		t.Fatalf("sub-agent = %s, want agent_weather", invoker.requests[1].AgentID)
	}
	if !strings.Contains(invoker.requests[0].RuntimeSystemPrompt, "Runtime Action Planning Contract") {
		t.Fatalf("planning system prompt does not include runtime contract: %s", invoker.requests[0].RuntimeSystemPrompt)
	}
	if strings.Contains(invoker.requests[0].RuntimeSystemPrompt, "Route weather requests to Weather Agent") {
		t.Fatalf("planning runtime prompt should not duplicate supervisor prompt: %s", invoker.requests[0].RuntimeSystemPrompt)
	}
	if strings.Contains(invoker.requests[0].Task, "Route weather requests to Weather Agent") {
		t.Fatalf("planning task should not include supervisor instructions: %s", invoker.requests[0].Task)
	}
	if !strings.Contains(invoker.requests[0].Task, "user_request") {
		t.Fatalf("planning task should carry user request, got: %s", invoker.requests[0].Task)
	}
	if disabled, _ := invoker.requests[0].Payload[orchestration.RuntimeDisableToolsPayloadKey].(bool); !disabled {
		t.Fatalf("planning should disable direct tool calling")
	}
	if !strings.Contains(invoker.requests[2].RuntimeSystemPrompt, "Runtime Final Answer Contract") {
		t.Fatalf("final system prompt does not include runtime contract: %s", invoker.requests[2].RuntimeSystemPrompt)
	}
	if strings.Contains(invoker.requests[2].RuntimeSystemPrompt, "Route weather requests to Weather Agent") {
		t.Fatalf("final runtime prompt should not duplicate supervisor prompt: %s", invoker.requests[2].RuntimeSystemPrompt)
	}
	if strings.Contains(invoker.requests[2].Task, "Route weather requests to Weather Agent") {
		t.Fatalf("final task should not include supervisor instructions: %s", invoker.requests[2].Task)
	}
	if !strings.Contains(invoker.requests[2].Task, "action_observations") {
		t.Fatalf("final task should carry action observations, got: %s", invoker.requests[2].Task)
	}
	nodeRuns, err := store.ListNodeRuns(context.Background(), "tenant_1", run.ID)
	if err != nil {
		t.Fatalf("ListNodeRuns() error = %v", err)
	}
	if len(nodeRuns) != 3 {
		t.Fatalf("node runs = %d, want 3", len(nodeRuns))
	}
}

func TestSupervisorRunnerDerivesAgentsFromGraph(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	catalog := fakeSupervisorAgentCatalog{
		"agent_supervisor": {ID: "agent_supervisor", TenantID: "tenant_1", Name: "Supervisor", Status: agent.StatusEnabled},
		"agent_weather":    {ID: "agent_weather", TenantID: "tenant_1", Name: "Weather Agent", Description: "Answer weather questions.", Status: agent.StatusEnabled},
	}
	invoker := &sequenceAgentInvoker{
		results: []ports.AgentInvocationResult{
			responseWithText(`{"actions":[{"type":"agent","agent_id":"agent_weather","task":"查询上海天气","reason":"weather question"}]}`),
			responseWithText(`{"actions":[],"final_answer_if_no_action":"上海今天多云。"}`),
			responseWithText("上海今天多云。"),
		},
	}
	runner := NewSupervisorRunner(slog.Default(), store, invoker, catalog, nil)

	run, err := runner.Execute(context.Background(), agentflow.Spec{
		ID:                "flow_graph_supervisor",
		TenantID:          tenant.ID("tenant_1"),
		Name:              "Graph Supervisor",
		Status:            agentflow.StatusEnabled,
		OrchestrationMode: agentflow.OrchestrationModeSupervisor,
		Graph: domain.FlowGraph{
			ID:          "flow_graph_supervisor",
			TenantID:    "tenant_1",
			Name:        "Graph Supervisor",
			Status:      domain.FlowStatusEnabled,
			EntryNodeID: "start",
			Nodes: map[id.ID]domain.Node{
				"start": {
					ID:   "start",
					Type: domain.NodeTypeStart,
					Name: "Start",
				},
				"supervisor": {
					ID:   "supervisor",
					Type: domain.NodeTypeSupervisor,
					Name: "Supervisor",
					Config: map[string]any{
						"agent_id":   "agent_supervisor",
						"agent_mode": "local",
					},
				},
				"weather": {
					ID:   "weather",
					Type: domain.NodeTypeAgent,
					Name: "Weather Agent",
					Config: map[string]any{
						"agent_id": "agent_weather",
					},
				},
			},
			Edges: []domain.Edge{
				{
					ID:         "start-supervisor",
					FromNodeID: "start",
					ToNodeID:   "supervisor",
					Type:       domain.EdgeTypeDefault,
				},
				{
					ID:         "supervisor-weather",
					FromNodeID: "supervisor",
					ToNodeID:   "weather",
					Type:       domain.EdgeTypeDefault,
				},
			},
		},
		Version: "v1",
	}, map[string]any{"message": "上海天气怎么样"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run status = %s, want succeeded", run.Status)
	}
	if invoker.requests[0].AgentID != "agent_supervisor" {
		t.Fatalf("planning agent = %s, want agent_supervisor", invoker.requests[0].AgentID)
	}
	if invoker.requests[1].AgentID != "agent_weather" {
		t.Fatalf("sub-agent = %s, want agent_weather", invoker.requests[1].AgentID)
	}
}

func TestSupervisorRunnerPlansAndExecutesToolAction(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	catalog := fakeSupervisorAgentCatalog{
		"agent_supervisor": {
			ID:       "agent_supervisor",
			TenantID: "tenant_1",
			Name:     "Supervisor",
			Status:   agent.StatusEnabled,
			ToolIDs:  []id.ID{"tool_search"},
		},
	}
	capabilities := fakeAgentCapabilityCatalog{
		"agent_supervisor": {
			Agent: catalog["agent_supervisor"],
			Tools: []tool.Spec{
				{
					ID:             "tool_search",
					TenantID:       "tenant_1",
					Name:           "tavily_search",
					Description:    "Search the web.",
					LLMDescription: "Search current web information.",
					Status:         tool.StatusEnabled,
					Implementation: tool.ImplementationConnector,
				},
			},
		},
	}
	invoker := &sequenceAgentInvoker{
		results: []ports.AgentInvocationResult{
			responseWithText(`{"actions":[{"type":"tool","tool_id":"tool_search","input":{"query":"AI news"},"reason":"need current information"}]}`),
			responseWithToolResult("工具执行完成。", tool.Result{
				ToolID:  "tool_search",
				Success: true,
				Data:    map[string]any{"summary": "AI news result"},
			}),
			responseWithText("AI news result"),
		},
	}
	runner := NewSupervisorRunner(slog.Default(), store, invoker, catalog, nil).WithAgentCapabilityCatalog(capabilities)

	run, err := runner.Execute(context.Background(), agentflow.Spec{
		ID:                "flow_tool_action",
		TenantID:          tenant.ID("tenant_1"),
		Name:              "Tool Action Flow",
		Status:            agentflow.StatusEnabled,
		OrchestrationMode: agentflow.OrchestrationModeSupervisor,
		Supervisor: agentflow.SupervisorSpec{
			SupervisorAgentID: "agent_supervisor",
			MaxDepth:          1,
			MaxSubAgentCalls:  3,
		},
		Graph: domain.FlowGraph{
			ID:          "flow_tool_action",
			TenantID:    "tenant_1",
			Name:        "Tool Action Flow",
			Status:      domain.FlowStatusEnabled,
			EntryNodeID: "start",
			Nodes: map[id.ID]domain.Node{
				"start": {ID: "start", Type: domain.NodeTypeStart, Name: "Start"},
				"supervisor": {
					ID:   "supervisor",
					Type: domain.NodeTypeSupervisor,
					Name: "Supervisor",
					Config: map[string]any{
						"agent_id":   "agent_supervisor",
						"agent_mode": "local",
					},
				},
			},
			Edges: []domain.Edge{
				{ID: "start-supervisor", FromNodeID: "start", ToNodeID: "supervisor", Type: domain.EdgeTypeDefault},
			},
		},
		Version: "v1",
	}, map[string]any{"message": "搜索今天 AI 新闻"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Output["text"] != "AI news result" {
		t.Fatalf("final text = %v", run.Output["text"])
	}
	if len(invoker.requests) != 3 {
		t.Fatalf("agent invocations = %d, want 3", len(invoker.requests))
	}
	if !strings.Contains(invoker.requests[0].RuntimeSystemPrompt, "Available Tool Actions") ||
		!strings.Contains(invoker.requests[0].RuntimeSystemPrompt, "tool_search") {
		t.Fatalf("planning prompt should list available tool actions: %s", invoker.requests[0].RuntimeSystemPrompt)
	}
	if got := invoker.requests[1].Payload["tool_id"]; got != "tool_search" {
		t.Fatalf("tool action payload tool_id = %v, want tool_search", got)
	}
	args, _ := invoker.requests[1].Payload["tool_args"].(map[string]any)
	if args["query"] != "AI news" {
		t.Fatalf("tool args = %#v, want query AI news", args)
	}
	if !strings.Contains(invoker.requests[2].Task, "action_observations") {
		t.Fatalf("final task should contain action observations, got %s", invoker.requests[2].Task)
	}
}

func TestSupervisorRunnerExecutesNestedAgentGraph(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	catalog := fakeSupervisorAgentCatalog{
		"agent_root":    {ID: "agent_root", TenantID: "tenant_1", Name: "Root Agent", Status: agent.StatusEnabled},
		"agent_search":  {ID: "agent_search", TenantID: "tenant_1", Name: "Search Agent", Description: "Search web information.", Status: agent.StatusEnabled},
		"agent_extract": {ID: "agent_extract", TenantID: "tenant_1", Name: "Extract Agent", Description: "Extract key facts.", Status: agent.StatusEnabled},
	}
	invoker := &sequenceAgentInvoker{
		results: []ports.AgentInvocationResult{
			responseWithText(`{"actions":[{"type":"agent","node_id":"search","agent_id":"agent_search","task":"搜索 AI 新闻","reason":"need search"}]}`),
			responseWithText(`{"actions":[{"type":"agent","node_id":"extract","agent_id":"agent_extract","task":"提取 AI 新闻重点","reason":"need extraction"}]}`),
			responseWithText(`{"actions":[],"final_answer_if_no_action":"提取到三条重点新闻。"}`),
			responseWithText("搜索结果重点：提取到三条重点新闻。"),
			responseWithText("最终总结：今天 AI 新闻有三条重点。"),
		},
	}
	runner := NewSupervisorRunner(slog.Default(), store, invoker, catalog, nil)

	run, err := runner.Execute(context.Background(), agentflow.Spec{
		ID:                "flow_nested_agent_graph",
		TenantID:          tenant.ID("tenant_1"),
		Name:              "Nested Agent Graph",
		Status:            agentflow.StatusEnabled,
		OrchestrationMode: agentflow.OrchestrationModeSupervisor,
		Supervisor: agentflow.SupervisorSpec{
			MaxDepth:         2,
			MaxSubAgentCalls: 3,
		},
		Graph: domain.FlowGraph{
			ID:          "flow_nested_agent_graph",
			TenantID:    "tenant_1",
			Name:        "Nested Agent Graph",
			Status:      domain.FlowStatusEnabled,
			EntryNodeID: "start",
			Nodes: map[id.ID]domain.Node{
				"start": {
					ID:   "start",
					Type: domain.NodeTypeStart,
					Name: "Start",
				},
				"root": {
					ID:   "root",
					Type: domain.NodeTypeSupervisor,
					Name: "Root Agent",
					Config: map[string]any{
						"agent_id":   "agent_root",
						"agent_mode": "local",
					},
				},
				"search": {
					ID:   "search",
					Type: domain.NodeTypeAgent,
					Name: "Search Agent",
					Config: map[string]any{
						"agent_id":   "agent_search",
						"agent_mode": "local",
					},
				},
				"extract": {
					ID:   "extract",
					Type: domain.NodeTypeAgent,
					Name: "Extract Agent",
					Config: map[string]any{
						"agent_id":   "agent_extract",
						"agent_mode": "existing",
					},
				},
			},
			Edges: []domain.Edge{
				{ID: "start-root", FromNodeID: "start", ToNodeID: "root", Type: domain.EdgeTypeDefault},
				{ID: "root-search", FromNodeID: "root", ToNodeID: "search", Type: domain.EdgeTypeDefault},
				{ID: "search-extract", FromNodeID: "search", ToNodeID: "extract", Type: domain.EdgeTypeDefault},
			},
		},
		Version: "v1",
	}, map[string]any{"message": "帮我搜索今天的 AI 新闻并总结重点"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Output["text"] != "最终总结：今天 AI 新闻有三条重点。" {
		t.Fatalf("final text = %v", run.Output["text"])
	}
	if len(invoker.requests) != 5 {
		t.Fatalf("agent invocations = %d, want 5", len(invoker.requests))
	}
	wantAgents := []id.ID{"agent_root", "agent_search", "agent_extract", "agent_search", "agent_root"}
	for index, want := range wantAgents {
		if invoker.requests[index].AgentID != want {
			t.Fatalf("request %d agent = %s, want %s", index, invoker.requests[index].AgentID, want)
		}
	}
	if !strings.Contains(invoker.requests[0].Task, "user_request") {
		t.Fatalf("root planning task should carry user request, got: %s", invoker.requests[0].Task)
	}
	if !strings.Contains(invoker.requests[1].Task, "user_request") {
		t.Fatalf("nested planning task should carry user request, got: %s", invoker.requests[1].Task)
	}
	nodeRuns, err := store.ListNodeRuns(context.Background(), "tenant_1", run.ID)
	if err != nil {
		t.Fatalf("ListNodeRuns() error = %v", err)
	}
	if len(nodeRuns) != 5 {
		t.Fatalf("node runs = %d, want 5", len(nodeRuns))
	}
}

func TestSupervisorRunnerRejectsExistingAgentWithChildren(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	catalog := fakeSupervisorAgentCatalog{
		"agent_supervisor": {ID: "agent_supervisor", TenantID: "tenant_1", Name: "Supervisor", Status: agent.StatusEnabled},
		"agent_weather":    {ID: "agent_weather", TenantID: "tenant_1", Name: "Weather Agent", Status: agent.StatusEnabled},
	}
	runner := NewSupervisorRunner(slog.Default(), store, &sequenceAgentInvoker{}, catalog, nil)

	_, err := runner.Execute(context.Background(), agentflow.Spec{
		ID:                "flow_invalid_existing_parent",
		TenantID:          tenant.ID("tenant_1"),
		Name:              "Invalid Existing Parent",
		Status:            agentflow.StatusEnabled,
		OrchestrationMode: agentflow.OrchestrationModeSupervisor,
		Graph: domain.FlowGraph{
			ID:          "flow_invalid_existing_parent",
			TenantID:    "tenant_1",
			Name:        "Invalid Existing Parent",
			Status:      domain.FlowStatusEnabled,
			EntryNodeID: "start",
			Nodes: map[id.ID]domain.Node{
				"start": {
					ID:   "start",
					Type: domain.NodeTypeStart,
					Name: "Start",
				},
				"supervisor": {
					ID:   "supervisor",
					Type: domain.NodeTypeSupervisor,
					Name: "Existing Supervisor",
					Config: map[string]any{
						"agent_id":   "agent_supervisor",
						"agent_mode": "existing",
					},
				},
				"weather": {
					ID:   "weather",
					Type: domain.NodeTypeAgent,
					Name: "Weather Agent",
					Config: map[string]any{
						"agent_id":   "agent_weather",
						"agent_mode": "existing",
					},
				},
			},
			Edges: []domain.Edge{
				{ID: "start-supervisor", FromNodeID: "start", ToNodeID: "supervisor", Type: domain.EdgeTypeDefault},
				{ID: "supervisor-weather", FromNodeID: "supervisor", ToNodeID: "weather", Type: domain.EdgeTypeDefault},
			},
		},
		Version: "v1",
	}, map[string]any{"message": "上海天气怎么样"})
	if err == nil {
		t.Fatal("expected existing agent with child nodes to be rejected")
	}
	if !strings.Contains(err.Error(), "must be a leaf node") {
		t.Fatalf("expected leaf-node validation error, got %v", err)
	}
}

type fakeSupervisorAgentCatalog map[id.ID]agent.Profile

func (c fakeSupervisorAgentCatalog) GetAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Profile, error) {
	return c[agentID], nil
}

type fakeAgentCapabilityCatalog map[id.ID]ports.AgentCapabilityConfig

func (c fakeAgentCapabilityCatalog) LoadAgentCapabilityConfig(ctx context.Context, tenantID tenant.ID, agentID id.ID) (ports.AgentCapabilityConfig, error) {
	return c[agentID], nil
}

type sequenceAgentInvoker struct {
	requests []ports.AgentInvocationRequest
	results  []ports.AgentInvocationResult
}

func (i *sequenceAgentInvoker) InvokeAgent(ctx context.Context, request ports.AgentInvocationRequest) (ports.AgentInvocationResult, error) {
	i.requests = append(i.requests, request)
	if len(i.results) == 0 {
		return ports.AgentInvocationResult{}, nil
	}
	result := i.results[0]
	i.results = i.results[1:]
	return result, nil
}

func responseWithText(text string) ports.AgentInvocationResult {
	return ports.AgentInvocationResult{
		Text:    text,
		TraceID: id.New("trace").String(),
		Response: event.Response{
			TraceID: id.New("trace").String(),
			Actions: []event.Action{
				{Type: event.ActionSpeak, Text: text},
				{Type: event.ActionEndTurn},
			},
		},
	}
}

func responseWithToolResult(text string, result tool.Result) ports.AgentInvocationResult {
	return ports.AgentInvocationResult{
		Text:    text,
		TraceID: id.New("trace").String(),
		Response: event.Response{
			TraceID: id.New("trace").String(),
			Actions: []event.Action{
				{Type: event.ActionDisplayText, Text: text, ToolResult: &result},
				{Type: event.ActionSpeak, Text: text},
				{Type: event.ActionEndTurn},
			},
		},
	}
}
