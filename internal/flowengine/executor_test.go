package flowengine

import (
	"context"
	"testing"

	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestExecutorUsesWorkflowContextContract(t *testing.T) {
	t.Parallel()

	registry := NewNodeRegistry()
	registry.Register(workflow.NodeTypeConnectorOperation, fakeAPINodeExecutor{t: t})
	store := NewMemoryRunStore()
	executor := NewExecutor(nil, store, registry)

	spec := workflow.Spec{
		ID:       id.ID("wf_doc"),
		TenantID: tenant.ID("tenant_1"),
		Name:     "Doc Workflow",
		Status:   workflow.StatusEnabled,
		Profile:  workflow.ProfileToolWorkflow,
		Version:  "v1",
		Graph: workflow.Graph{
			EntryNodeID: "start",
			Nodes: map[id.ID]workflow.Node{
				"start": {ID: "start", Type: workflow.NodeTypeStart, Name: "Start"},
				"create_doc": {
					ID:   "create_doc",
					Type: workflow.NodeTypeConnectorOperation,
					Name: "Create Doc",
					Config: map[string]any{
						"connector_operation_id": "connop_create_doc",
						"response_alias":         "create_doc_step",
						"input_mapping": map[string]any{
							"title": "$flow_input.request.title",
						},
						"output_mapping": map[string]any{
							"document_id": "$.document.document_id",
						},
						"write_context": map[string]any{
							"variables.feishu.document_id": "$output.document_id",
							"variables.feishu.api_data":    "$.data.inner",
						},
					},
				},
				"end": {
					ID:   "end",
					Type: workflow.NodeTypeEnd,
					Name: "End",
					Config: map[string]any{
						"output_mapping": map[string]any{
							"document_id": "$variables.feishu.document_id",
						},
						"write_context": map[string]any{
							"flow_output.document_id": "$output.document_id",
						},
					},
				},
			},
			Edges: []workflow.Edge{
				{FromNodeID: "start", ToNodeID: "create_doc"},
				{FromNodeID: "create_doc", ToNodeID: "end"},
			},
		},
	}

	run, err := executor.Execute(context.Background(), spec, map[string]any{
		"request": map[string]any{"title": "AI News"},
	}, nil, "trace_1")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != workflow.RunStatusSucceeded {
		t.Fatalf("run status = %s", run.Status)
	}
	if got := readNested(run.Output, "document_id"); got != "doc_123" {
		t.Fatalf("expected flow output document_id, got %#v", got)
	}
	if got := readNested(run.Context, "variables", "feishu", "document_id"); got != "doc_123" {
		t.Fatalf("expected variable document_id, got %#v", got)
	}
	if got := readNested(run.Context, "variables", "feishu", "api_data"); got != "api_data_value" {
		t.Fatalf("expected API payload data field, got %#v", got)
	}
	if got := readNested(run.Context, "responses", "connector", "create_doc_step", "document", "document_id"); got != "doc_123" {
		t.Fatalf("expected connector response archive, got %#v", got)
	}
}

func TestRunContextRejectsReadOnlyDomainWrites(t *testing.T) {
	t.Parallel()

	ctx := NewRunContext(map[string]any{"query": "hello"}, nil)
	if err := ctx.Write("flow_input.query", "changed"); err == nil {
		t.Fatal("expected flow_input write to fail")
	}
	if err := ctx.Write("responses.connector.search", map[string]any{}); err == nil {
		t.Fatal("expected responses write to fail")
	}
	if err := ctx.Write("flow_output.answer", "ok"); err != nil {
		t.Fatalf("expected flow_output write to succeed: %v", err)
	}
	if got := readNested(ctx.Output(), "answer"); got != "ok" {
		t.Fatalf("expected flow_output answer, got %#v", got)
	}
	if err := ctx.Write("legacy.path", "still_supported"); err != nil {
		t.Fatalf("expected legacy bare context write to succeed: %v", err)
	}
	if got := readNested(ctx.Ctx, "legacy", "path"); got != "still_supported" {
		t.Fatalf("expected legacy context path, got %#v", got)
	}
}

func TestAgentNodeIgnoresOutputMappingAndWritesContextFromRawOutput(t *testing.T) {
	t.Parallel()

	ctx := NewRunContext(map[string]any{}, nil)
	result, err := applyOutputMappings(workflow.Node{
		ID:   "agent",
		Type: workflow.NodeTypeAgent,
		Config: map[string]any{
			"output_mapping": map[string]any{
				"user_request_detail": "$.text",
			},
			"write_context": map[string]any{
				"variables.user_request_detail": "$.user_request_detail",
				"flow_output.return_message":    "$.answer",
			},
		},
	}, ctx, map[string]any{
		"text":                "",
		"answer":              "route to web",
		"user_request_detail": "请搜索今天的 AI 热门话题",
	})
	if err != nil {
		t.Fatalf("applyOutputMappings() error = %v", err)
	}
	if result.Output["user_request_detail"] != "请搜索今天的 AI 热门话题" {
		t.Fatalf("agent output should remain raw output schema object, got %#v", result.Output)
	}
	if result.ContextWrites["variables.user_request_detail"] != "请搜索今天的 AI 热门话题" {
		t.Fatalf("write_context should read raw agent output, got %#v", result.ContextWrites)
	}
	if result.ContextWrites["flow_output.return_message"] != "route to web" {
		t.Fatalf("flow output write = %#v", result.ContextWrites)
	}
}

func TestExecutorRequiresExplicitNextNodeToBeConnected(t *testing.T) {
	t.Parallel()

	registry := NewNodeRegistry()
	registry.Register(workflow.NodeTypeTransform, fakeRoutingNodeExecutor{next: []id.ID{"missing"}})
	store := NewMemoryRunStore()
	executor := NewExecutor(nil, store, registry)

	spec := workflow.Spec{
		ID:       id.ID("wf_route"),
		TenantID: tenant.ID("tenant_1"),
		Name:     "Route Workflow",
		Status:   workflow.StatusEnabled,
		Profile:  workflow.ProfileAgentWorkflow,
		Version:  "v1",
		Graph: workflow.Graph{
			EntryNodeID: "start",
			Nodes: map[id.ID]workflow.Node{
				"start":  {ID: "start", Type: workflow.NodeTypeStart, Name: "Start"},
				"router": {ID: "router", Type: workflow.NodeTypeTransform, Name: "Router"},
				"end":    {ID: "end", Type: workflow.NodeTypeEnd, Name: "End"},
			},
			Edges: []workflow.Edge{
				{FromNodeID: "start", ToNodeID: "router"},
				{FromNodeID: "router", ToNodeID: "end"},
			},
		},
	}

	_, err := executor.Execute(context.Background(), spec, map[string]any{}, nil, "trace_route")
	if err == nil {
		t.Fatal("Execute() error = nil, want invalid next-node validation error")
	}
}

type fakeAPINodeExecutor struct {
	t *testing.T
}

type fakeRoutingNodeExecutor struct {
	next []id.ID
}

func (e fakeRoutingNodeExecutor) ExecuteNode(ctx context.Context, request NodeExecutionRequest) (NodeResult, error) {
	return NodeResult{Output: map[string]any{"routed": true}, NextNodeIDs: e.next}, nil
}

func (e fakeAPINodeExecutor) ExecuteNode(ctx context.Context, request NodeExecutionRequest) (NodeResult, error) {
	if request.Input["title"] != "AI News" {
		e.t.Fatalf("expected mapped title, got %#v", request.Input["title"])
	}
	return NodeResult{
		Output: map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"inner": "api_data_value",
				},
				"document": map[string]any{
					"document_id": "doc_123",
				},
			},
		},
	}, nil
}

func readNested(root map[string]any, parts ...string) any {
	var current any = root
	for _, part := range parts {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = asMap[part]
	}
	return current
}
