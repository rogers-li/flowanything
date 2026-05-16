package workflowruntime

import (
	"context"
	"testing"
	"time"

	"flow-anything/internal/flowengine"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestTransformRemoveFieldsRemovesMergeInfoRecursively(t *testing.T) {
	t.Parallel()

	output, err := ExecuteTransformFunction(context.Background(), "json.remove_fields", map[string]any{
		"value": []any{
			map[string]any{
				"block_type": "table",
				"merge_info": map[string]any{"row_span": 1},
				"children": []any{
					map[string]any{
						"text":       "cell",
						"merge_info": "readonly",
					},
				},
			},
		},
		"fields":    []any{"merge_info"},
		"recursive": true,
	})
	if err != nil {
		t.Fatalf("ExecuteTransformFunction() error = %v", err)
	}
	if got := output["removed_count"]; got != 2 {
		t.Fatalf("removed_count = %#v, want 2", got)
	}
	blocks, ok := output["result"].([]any)
	if !ok || len(blocks) != 1 {
		t.Fatalf("result = %#v, want one block", output["result"])
	}
	first := blocks[0].(map[string]any)
	if _, exists := first["merge_info"]; exists {
		t.Fatalf("top-level merge_info was not removed: %#v", first)
	}
	child := first["children"].([]any)[0].(map[string]any)
	if _, exists := child["merge_info"]; exists {
		t.Fatalf("nested merge_info was not removed: %#v", child)
	}
}

func TestTransformNodeIntegratesWithWorkflowMappings(t *testing.T) {
	t.Parallel()

	registry := flowengine.NewNodeRegistry()
	registry.Register(workflow.NodeTypeTransform, TransformNodeExecutor{})
	store := flowengine.NewMemoryRunStore()
	executor := flowengine.NewExecutor(nil, store, registry)

	spec := workflow.Spec{
		ID:       id.ID("wf_transform"),
		TenantID: tenant.ID("tenant_1"),
		Name:     "Transform Workflow",
		Status:   workflow.StatusEnabled,
		Profile:  workflow.ProfileToolWorkflow,
		Version:  "v1",
		Graph: workflow.Graph{
			EntryNodeID: "start",
			Nodes: map[id.ID]workflow.Node{
				"start": {ID: "start", Type: workflow.NodeTypeStart, Name: "Start"},
				"sanitize": {
					ID:   "sanitize",
					Type: workflow.NodeTypeTransform,
					Name: "Remove merge_info",
					Config: map[string]any{
						"function_id": "json.remove_fields",
						"input_mapping": map[string]any{
							"value":     "$flow_input.blocks",
							"fields":    []any{"merge_info"},
							"recursive": true,
						},
						"write_context": map[string]any{
							"variables.feishu.cleaned_blocks": "$.result",
							"flow_output.removed_count":       "$.removed_count",
						},
					},
				},
				"end": {ID: "end", Type: workflow.NodeTypeEnd, Name: "End"},
			},
			Edges: []workflow.Edge{
				{FromNodeID: "start", ToNodeID: "sanitize"},
				{FromNodeID: "sanitize", ToNodeID: "end"},
			},
		},
	}

	run, err := executor.Execute(context.Background(), spec, map[string]any{
		"blocks": []any{map[string]any{"merge_info": "readonly", "text": "ok"}},
	}, nil, "trace_transform")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := readNestedTransformTest(run.Output, "removed_count"); got != 1 {
		t.Fatalf("removed_count = %#v, want 1", got)
	}
	cleaned := readNestedTransformTest(run.Context, "variables", "feishu", "cleaned_blocks").([]any)[0].(map[string]any)
	if _, exists := cleaned["merge_info"]; exists {
		t.Fatalf("merge_info was not removed from context write: %#v", cleaned)
	}
}

func TestConditionNodeWritesContextAndStopsWhenBranchHasNoNextNode(t *testing.T) {
	t.Parallel()

	registry := flowengine.NewNodeRegistry()
	registry.Register(workflow.NodeTypeCondition, ConditionNodeExecutor{})
	store := flowengine.NewMemoryRunStore()
	executor := flowengine.NewExecutor(nil, store, registry)

	spec := workflow.Spec{
		ID:       id.ID("wf_condition_stop"),
		TenantID: tenant.ID("tenant_1"),
		Name:     "Condition Stop Workflow",
		Status:   workflow.StatusEnabled,
		Profile:  workflow.ProfileToolWorkflow,
		Version:  "v1",
		Graph: workflow.Graph{
			EntryNodeID: "start",
			Nodes: map[id.ID]workflow.Node{
				"start": {ID: "start", Type: workflow.NodeTypeStart, Name: "Start"},
				"check": {
					ID:   "check",
					Type: workflow.NodeTypeCondition,
					Name: "Check API Status",
					Config: map[string]any{
						"branches": []any{
							map[string]any{
								"id":   "failed",
								"name": "API Failed",
								"mode": "all",
								"rules": []any{
									map[string]any{
										"left":     "$flow_input.api.code",
										"operator": "not_equals",
										"right":    map[string]any{"type": "const", "value": 0},
									},
								},
								"write_context": map[string]any{
									"flow_output.code":    "$flow_input.api.code",
									"flow_output.message": "$flow_input.api.msg",
								},
								"next_node_id": "",
							},
						},
						"default_branch": map[string]any{
							"next_node_id": "end",
						},
					},
				},
				"end": {ID: "end", Type: workflow.NodeTypeEnd, Name: "End"},
			},
			Edges: []workflow.Edge{
				{FromNodeID: "start", ToNodeID: "check"},
				{FromNodeID: "check", ToNodeID: "end"},
			},
		},
	}

	run, err := executor.Execute(context.Background(), spec, map[string]any{
		"api": map[string]any{"code": 500, "msg": "upstream failed"},
	}, nil, "trace_condition_stop")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := readNestedTransformTest(run.Output, "code"); got != 500 {
		t.Fatalf("flow_output.code = %#v, want 500", got)
	}
	if got := readNestedTransformTest(run.Output, "message"); got != "upstream failed" {
		t.Fatalf("flow_output.message = %#v, want upstream failed", got)
	}
	nodeRuns, err := store.ListNodeRuns(context.Background(), tenant.ID("tenant_1"), run.ID)
	if err != nil {
		t.Fatalf("ListNodeRuns() error = %v", err)
	}
	for _, nodeRun := range nodeRuns {
		if nodeRun.NodeID == "end" {
			t.Fatalf("end node should not execute when matched branch has empty next_node_id")
		}
	}
}

func TestConditionNodeRoutesToDefaultBranch(t *testing.T) {
	t.Parallel()

	registry := flowengine.NewNodeRegistry()
	registry.Register(workflow.NodeTypeCondition, ConditionNodeExecutor{})
	store := flowengine.NewMemoryRunStore()
	executor := flowengine.NewExecutor(nil, store, registry)

	spec := workflow.Spec{
		ID:       id.ID("wf_condition_default"),
		TenantID: tenant.ID("tenant_1"),
		Name:     "Condition Default Workflow",
		Status:   workflow.StatusEnabled,
		Profile:  workflow.ProfileToolWorkflow,
		Version:  "v1",
		Graph: workflow.Graph{
			EntryNodeID: "start",
			Nodes: map[id.ID]workflow.Node{
				"start": {ID: "start", Type: workflow.NodeTypeStart, Name: "Start"},
				"check": {
					ID:   "check",
					Type: workflow.NodeTypeCondition,
					Name: "Check Status",
					Config: map[string]any{
						"branches": []any{
							map[string]any{
								"name": "Failed",
								"rules": []any{
									map[string]any{"left": "$flow_input.code", "operator": "not_equals", "right": map[string]any{"type": "const", "value": 0}},
								},
								"next_node_id": "",
							},
						},
						"default_branch": map[string]any{
							"write_context": map[string]any{
								"flow_output.status": map[string]any{"type": "const", "value": "ok"},
							},
							"next_node_id": "end",
						},
					},
				},
				"end": {ID: "end", Type: workflow.NodeTypeEnd, Name: "End"},
			},
			Edges: []workflow.Edge{
				{FromNodeID: "start", ToNodeID: "check"},
				{FromNodeID: "check", ToNodeID: "end"},
			},
		},
	}

	run, err := executor.Execute(context.Background(), spec, map[string]any{"code": 0}, nil, "trace_condition_default")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := readNestedTransformTest(run.Output, "status"); got != "ok" {
		t.Fatalf("flow_output.status = %#v, want ok", got)
	}
}

func TestConnectorNodePublishesFailedAPIResultForConditionHandling(t *testing.T) {
	t.Parallel()

	registry := flowengine.NewNodeRegistry()
	registry.Register(workflow.NodeTypeConnectorOperation, NewConnectorOperationNodeExecutor(fakeConnectorInvoker{
		result: connector.InvokeResult{
			RequestID:  "connreq_failed",
			Success:    false,
			ErrorCode:  "http_status_400",
			Data:       map[string]any{"status_code": 400, "msg": "bad request"},
			FinishedAt: time.Now().UTC(),
		},
	}))
	registry.Register(workflow.NodeTypeCondition, ConditionNodeExecutor{})
	store := flowengine.NewMemoryRunStore()
	executor := flowengine.NewExecutor(nil, store, registry)

	spec := workflow.Spec{
		ID:       id.ID("wf_connector_failure"),
		TenantID: tenant.ID("tenant_1"),
		Name:     "Connector Failure Workflow",
		Status:   workflow.StatusEnabled,
		Profile:  workflow.ProfileToolWorkflow,
		Version:  "v1",
		Graph: workflow.Graph{
			EntryNodeID: "start",
			Nodes: map[id.ID]workflow.Node{
				"start": {ID: "start", Type: workflow.NodeTypeStart, Name: "Start"},
				"call_api": {
					ID:   "call_api",
					Type: workflow.NodeTypeConnectorOperation,
					Name: "Call API",
					Config: map[string]any{
						"connector_operation_id": "connop_may_fail",
						"response_alias":         "may_fail",
					},
				},
				"check": {
					ID:   "check",
					Type: workflow.NodeTypeCondition,
					Name: "Check API Result",
					Config: map[string]any{
						"branches": []any{
							map[string]any{
								"name": "API Failed",
								"rules": []any{
									map[string]any{
										"left":     "$connector_response.may_fail.success",
										"operator": "equals",
										"right":    false,
									},
								},
								"write_context": map[string]any{
									"flow_output.code":    "$connector_response.may_fail.error_code",
									"flow_output.message": "$connector_response.may_fail.data.msg",
								},
								"next_node_id": "",
							},
						},
					},
				},
				"end": {ID: "end", Type: workflow.NodeTypeEnd, Name: "End"},
			},
			Edges: []workflow.Edge{
				{FromNodeID: "start", ToNodeID: "call_api"},
				{FromNodeID: "call_api", ToNodeID: "check"},
				{FromNodeID: "check", ToNodeID: "end"},
			},
		},
	}

	run, err := executor.Execute(context.Background(), spec, nil, nil, "trace_connector_failure")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != workflow.RunStatusSucceeded {
		t.Fatalf("run status = %s, want succeeded", run.Status)
	}
	if got := readNestedTransformTest(run.Context, "responses", "connector", "may_fail", "success"); got != false {
		t.Fatalf("connector failure should be archived in context, got %#v", got)
	}
	if got := readNestedTransformTest(run.Output, "code"); got != "http_status_400" {
		t.Fatalf("flow_output.code = %#v, want http_status_400", got)
	}
	if got := readNestedTransformTest(run.Output, "message"); got != "bad request" {
		t.Fatalf("flow_output.message = %#v, want bad request", got)
	}
}

type fakeConnectorInvoker struct {
	result connector.InvokeResult
	err    error
}

func (f fakeConnectorInvoker) Invoke(ctx context.Context, req connector.InvokeRequest) (connector.InvokeResult, error) {
	return f.result, f.err
}

func readNestedTransformTest(root map[string]any, parts ...string) any {
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
