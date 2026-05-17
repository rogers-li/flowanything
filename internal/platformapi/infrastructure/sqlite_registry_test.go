package infrastructure

import (
	"context"
	"testing"

	flowdomain "flow-anything/internal/agentflow/domain"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestSQLiteRegistryStoresResources(t *testing.T) {
	t.Parallel()

	registry, err := OpenSQLiteRegistry(context.Background(), "file:test_registry?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("OpenSQLiteRegistry() error = %v", err)
	}
	defer registry.Close()

	tenantID := tenant.ID("tenant_1")
	operationID := id.ID("connop_query_order")
	toolID := id.ID("tool_query_order")
	skillID := id.ID("skill_order")
	flowID := id.ID("flow_support")
	workflowID := id.ID("wf_upload_doc")

	if err := registry.SaveConnectorOperation(context.Background(), connector.OperationSpec{
		ID:                 operationID,
		TenantID:           tenantID,
		Name:               "query_order",
		BusinessDomain:     "Order",
		OwnerTeam:          "Order Platform",
		ImplementationMode: connector.ImplementationModeSimpleHTTP,
		Type:               connector.OperationTypeHTTP,
		BaseURL:            "http://orders.test",
		Method:             "GET",
		Path:               "/orders/{order_id}",
		TimeoutMillis:      5000,
		Version:            "v1",
	}); err != nil {
		t.Fatalf("SaveConnectorOperation() error = %v", err)
	}

	if err := registry.SaveTool(context.Background(), tool.Spec{
		ID:             toolID,
		TenantID:       tenantID,
		Name:           "query_order",
		Implementation: tool.ImplementationConnector,
		Binding: tool.Binding{
			ConnectorOperationID: operationID,
		},
		SideEffect:    tool.SideEffectRead,
		RiskLevel:     tool.RiskLow,
		TimeoutMillis: 5000,
		Version:       "v1",
	}); err != nil {
		t.Fatalf("SaveTool() error = %v", err)
	}

	if err := registry.SaveSkill(context.Background(), skill.Spec{
		ID:       skillID,
		TenantID: tenantID,
		Name:     "order",
		ToolIDs:  []id.ID{toolID},
		Version:  "v1",
	}); err != nil {
		t.Fatalf("SaveSkill() error = %v", err)
	}

	if err := registry.SaveAgent(context.Background(), agent.Profile{
		ID:       id.ID("agent_order"),
		TenantID: tenantID,
		Name:     "Order Agent",
		SkillIDs: []id.ID{skillID},
		Version:  "v1",
	}); err != nil {
		t.Fatalf("SaveAgent() error = %v", err)
	}

	if err := registry.SaveAgentFlow(context.Background(), agentflow.Spec{
		ID:       flowID,
		TenantID: tenantID,
		Name:     "Support Flow",
		Status:   agentflow.StatusEnabled,
		Graph: flowdomain.FlowGraph{
			ID:          flowID,
			TenantID:    tenantID,
			Name:        "Support Flow",
			Status:      flowdomain.FlowStatusEnabled,
			EntryNodeID: "start",
			Nodes: map[id.ID]flowdomain.Node{
				"start": {ID: "start", Type: flowdomain.NodeTypeStart, Name: "Start"},
			},
		},
		InputSchema: map[string]any{
			"type": "object",
		},
		OutputSchema: map[string]any{
			"type": "object",
		},
		Version: "v1",
	}); err != nil {
		t.Fatalf("SaveAgentFlow() error = %v", err)
	}

	if err := registry.SaveWorkflow(context.Background(), workflow.Spec{
		ID:       workflowID,
		TenantID: tenantID,
		Name:     "Upload Doc",
		Status:   workflow.StatusEnabled,
		Profile:  workflow.ProfileToolWorkflow,
		Graph: workflow.Graph{
			EntryNodeID: "start",
			Nodes: map[id.ID]workflow.Node{
				"start": {ID: "start", Type: workflow.NodeTypeStart, Name: "Start"},
			},
		},
		ContextSchema: map[string]any{"type": "object"},
		Version:       "v1",
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	storedTool, err := registry.GetTool(context.Background(), tenantID, toolID)
	if err != nil {
		t.Fatalf("GetTool() error = %v", err)
	}
	if storedTool.Binding.ConnectorOperationID != operationID {
		t.Fatalf("expected connector operation id %q, got %q", operationID, storedTool.Binding.ConnectorOperationID)
	}
	storedOperation, err := registry.GetConnectorOperation(context.Background(), tenantID, operationID)
	if err != nil {
		t.Fatalf("GetConnectorOperation() error = %v", err)
	}
	if storedOperation.BusinessDomain != "Order" || storedOperation.OwnerTeam != "Order Platform" {
		t.Fatalf("expected operation metadata to be stored, got %#v", storedOperation)
	}
	if storedOperation.ImplementationMode != connector.ImplementationModeSimpleHTTP {
		t.Fatalf("expected implementation mode %q, got %q", connector.ImplementationModeSimpleHTTP, storedOperation.ImplementationMode)
	}

	agents, err := registry.ListAgents(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	flows, err := registry.ListAgentFlows(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListAgentFlows() error = %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("expected 1 agent flow, got %d", len(flows))
	}
	if flows[0].Graph.EntryNodeID != "start" {
		t.Fatalf("expected graph to roundtrip, got %#v", flows[0].Graph)
	}
	workflows, err := registry.ListWorkflows(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if len(workflows) != 1 || workflows[0].ID != workflowID {
		t.Fatalf("expected workflow to roundtrip, got %#v", workflows)
	}
}
