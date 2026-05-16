package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/infrastructure"
	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestExecutorRunsSequentialGraph(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	recorder := &recordingExecutor{}
	executor := NewExecutor(slog.Default(), store, NewNodeRegistry(), nil)
	executor.RegisterNodeExecutor("record", recorder)

	graph := testGraph(map[id.ID]domain.Node{
		"start": {ID: "start", Type: domain.NodeTypeStart, Name: "Start"},
		"a":     {ID: "a", Type: "record", Name: "A"},
		"b":     {ID: "b", Type: "record", Name: "B"},
	}, []domain.Edge{
		{FromNodeID: "start", ToNodeID: "a"},
		{FromNodeID: "a", ToNodeID: "b"},
	})

	run, err := executor.Execute(context.Background(), graph, map[string]any{"request": "hello"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusSucceeded)
	}
	if got := recorder.order(); fmt.Sprint(got) != "[a b]" {
		t.Fatalf("execution order = %v, want [a b]", got)
	}
}

func TestExecutorRoutesByCondition(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	recorder := &recordingExecutor{}
	executor := NewExecutor(slog.Default(), store, NewNodeRegistry(), nil)
	executor.RegisterNodeExecutor("record", recorder)

	graph := testGraph(map[id.ID]domain.Node{
		"start":  {ID: "start", Type: domain.NodeTypeStart, Name: "Start"},
		"router": {ID: "router", Type: "record", Name: "Router"},
		"left":   {ID: "left", Type: "record", Name: "Left"},
		"right":  {ID: "right", Type: "record", Name: "Right"},
	}, []domain.Edge{
		{FromNodeID: "start", ToNodeID: "router"},
		{FromNodeID: "router", ToNodeID: "left", Type: domain.EdgeTypeConditional, Condition: &domain.EdgeCondition{Path: "input.route", Equals: "left"}},
		{FromNodeID: "router", ToNodeID: "right", Type: domain.EdgeTypeConditional, Condition: &domain.EdgeCondition{Path: "input.route", Equals: "right"}},
	})

	run, err := executor.Execute(context.Background(), graph, map[string]any{"route": "right"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusSucceeded)
	}
	if got := recorder.order(); fmt.Sprint(got) != "[router right]" {
		t.Fatalf("execution order = %v, want [router right]", got)
	}
}

func TestExecutorWaitsForParallelBranchesBeforeJoin(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	recorder := &recordingExecutor{delays: map[id.ID]time.Duration{"a": 30 * time.Millisecond}}
	executor := NewExecutor(slog.Default(), store, NewNodeRegistry(), nil)
	executor.RegisterNodeExecutor("record", recorder)

	graph := testGraph(map[id.ID]domain.Node{
		"start": {ID: "start", Type: domain.NodeTypeStart, Name: "Start"},
		"fork":  {ID: "fork", Type: "record", Name: "Fork"},
		"a":     {ID: "a", Type: "record", Name: "A"},
		"b":     {ID: "b", Type: "record", Name: "B"},
		"join":  {ID: "join", Type: "record", Name: "Join"},
	}, []domain.Edge{
		{FromNodeID: "start", ToNodeID: "fork"},
		{FromNodeID: "fork", ToNodeID: "a"},
		{FromNodeID: "fork", ToNodeID: "b"},
		{FromNodeID: "a", ToNodeID: "join"},
		{FromNodeID: "b", ToNodeID: "join"},
	})

	run, err := executor.Execute(context.Background(), graph, map[string]any{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusSucceeded)
	}
	order := recorder.order()
	if len(order) != 4 || order[3] != "join" {
		t.Fatalf("execution order = %v, want join after both branches", order)
	}
}

func TestExecutorRunsWorkflowAgentNodeWithContextMappings(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	invoker := &fakeAgentInvoker{
		result: ports.AgentInvocationResult{
			Text: `{"summary":"searched AI news","answer":"今日 AI 新闻重点已汇总"}`,
		},
	}
	executor := NewExecutor(slog.Default(), store, NewNodeRegistry(), nil)
	executor.RegisterNodeExecutor(domain.NodeTypeAgent, NewAgentNodeExecutor(invoker))

	graph := testGraph(map[id.ID]domain.Node{
		"start": {ID: "start", Type: domain.NodeTypeStart, Name: "Start"},
		"agent": {
			ID:   "agent",
			Type: domain.NodeTypeAgent,
			Name: "Knowledge Agent",
			Config: map[string]any{
				"agent_id":    "agent_knowledge",
				"task_path":   "input.task",
				"output_mode": "json",
				"input_mapping": map[string]any{
					"task": "$flow_input.request",
				},
				"write_context": map[string]any{
					"variables.summary":  "$.summary",
					"flow_output.answer": "$.answer",
				},
			},
		},
	}, []domain.Edge{
		{FromNodeID: "start", ToNodeID: "agent"},
	})

	run, err := executor.Execute(context.Background(), graph, map[string]any{"request": "帮我搜索今天的 AI 新闻"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if invoker.request.Task != "帮我搜索今天的 AI 新闻" {
		t.Fatalf("Task = %q, want mapped workflow input", invoker.request.Task)
	}
	if run.Output["answer"] != "今日 AI 新闻重点已汇总" {
		t.Fatalf("run output answer = %v, want mapped flow output", run.Output["answer"])
	}
}

func TestExecutorContinuesAgentDirectedWorkflowWithMappedUserTask(t *testing.T) {
	store := infrastructure.NewMemoryRunStore()
	invoker := &routingAgentInvoker{}
	executor := NewExecutor(slog.Default(), store, NewNodeRegistry(), nil)
	executor.RegisterNodeExecutor(domain.NodeTypeAgent, NewAgentNodeExecutor(invoker))

	graph := testGraph(map[id.ID]domain.Node{
		"start": {ID: "start", Type: domain.NodeTypeStart, Name: "Start"},
		"router": {
			ID:   "router",
			Type: domain.NodeTypeAgent,
			Name: "Router Agent",
			Config: map[string]any{
				"agent_id":            "agent_router",
				"agent_routing_mode":  "agent_directed",
				"output_mode":         "json",
				"input_mapping":       map[string]any{"user_request": "$flow_input.user_request"},
				"output_schema":       map[string]any{"properties": map[string]any{"user_request_detail": map[string]any{"type": "string"}}},
				"write_context":       map[string]any{"variables.user_request_detail": "$.user_request_detail"},
				"parse_json_output":   true,
				"agent_directed_test": true,
			},
		},
		"web": {
			ID:   "web",
			Type: domain.NodeTypeAgent,
			Name: "Web Search Agent",
			Config: map[string]any{
				"agent_id":      "agent_web",
				"input_mapping": map[string]any{"user_task": "$variables.user_request_detail"},
				"write_context": map[string]any{"flow_output.return_message": "$.text"},
			},
		},
	}, []domain.Edge{
		{FromNodeID: "start", ToNodeID: "router"},
		{FromNodeID: "router", ToNodeID: "web"},
	})

	run, err := executor.Execute(context.Background(), graph, map[string]any{"user_request": "请搜索今天的AI热门话题"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusSucceeded)
	}
	tasks := invoker.tasks()
	if len(tasks) != 2 {
		t.Fatalf("agent tasks = %v, want router and web tasks", tasks)
	}
	if !strings.Contains(tasks[0], "user_request") || !strings.Contains(tasks[0], "请搜索今天的AI热门话题") {
		t.Fatalf("router task = %q, want structured user_request input", tasks[0])
	}
	if !strings.Contains(tasks[1], "user_task") || !strings.Contains(tasks[1], "请搜索今天的AI热门话题") {
		t.Fatalf("web task = %q, want structured user_task input", tasks[1])
	}
	if run.Output["return_message"] != "web search result" {
		t.Fatalf("run output return_message = %v, want web search result", run.Output["return_message"])
	}
}

type recordingExecutor struct {
	mu     sync.Mutex
	nodes  []id.ID
	delays map[id.ID]time.Duration
}

func (e *recordingExecutor) ExecuteNode(ctx context.Context, request ports.NodeExecutionRequest) (domain.NodeResult, error) {
	if delay := e.delays[request.Node.ID]; delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return domain.NodeResult{}, ctx.Err()
		}
	}
	e.mu.Lock()
	e.nodes = append(e.nodes, request.Node.ID)
	e.mu.Unlock()
	return domain.NodeResult{
		Output:    map[string]any{"node": request.Node.ID.String()},
		Variables: map[string]any{request.Node.ID.String(): true},
	}, nil
}

func (e *recordingExecutor) order() []id.ID {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]id.ID, len(e.nodes))
	copy(result, e.nodes)
	return result
}

type routingAgentInvoker struct {
	mu       sync.Mutex
	requests []ports.AgentInvocationRequest
}

func (i *routingAgentInvoker) InvokeAgent(ctx context.Context, request ports.AgentInvocationRequest) (ports.AgentInvocationResult, error) {
	i.mu.Lock()
	i.requests = append(i.requests, request)
	i.mu.Unlock()
	if request.AgentID == "agent_router" {
		return ports.AgentInvocationResult{
			Text: `{"answer":"route to web","next_node_ids":["web"],"reason":"needs search","user_request_detail":"请搜索今天的AI热门话题"}`,
		}, nil
	}
	return ports.AgentInvocationResult{Text: "web search result"}, nil
}

func (i *routingAgentInvoker) tasks() []string {
	i.mu.Lock()
	defer i.mu.Unlock()
	result := make([]string, 0, len(i.requests))
	for _, request := range i.requests {
		result = append(result, request.Task)
	}
	return result
}

func testGraph(nodes map[id.ID]domain.Node, edges []domain.Edge) domain.FlowGraph {
	return domain.FlowGraph{
		ID:          "flow_test",
		TenantID:    tenant.ID("tenant_1"),
		Name:        "Test Flow",
		Status:      domain.FlowStatusEnabled,
		Version:     "v1",
		EntryNodeID: "start",
		Nodes:       nodes,
		Edges:       edges,
		Policy: domain.ExecutionPolicy{
			MaxSteps:       16,
			MaxParallelism: 8,
		},
	}
}
