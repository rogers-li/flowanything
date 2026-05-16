package application

import (
	"context"
	"io"
	"log/slog"
	"strings"
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
	"flow-anything/internal/platformapi/infrastructure"
)

func TestCreateAndListAgents(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := infrastructure.NewMemoryAgentRepository()
	service := New(
		logger,
		repo,
		infrastructure.NewMemoryAgentFlowRepository(),
		infrastructure.NewMemoryToolRepository(),
		infrastructure.NewMemorySkillRepository(),
		infrastructure.NewMemoryConnectorRepository(),
		infrastructure.NewMemoryConnectorOperationRepository(),
	)

	created, err := service.CreateAgent(context.Background(), agent.Profile{
		TenantID: tenant.ID("tenant_1"),
		Name:     "Demo Agent",
	})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if created.ID.Empty() {
		t.Fatal("expected generated agent id")
	}
	if created.Status != agent.StatusDraft {
		t.Fatalf("expected draft agent, got %q", created.Status)
	}
	if created.DefaultLang != "zh-CN" {
		t.Fatalf("expected default language zh-CN, got %q", created.DefaultLang)
	}

	agents, err := service.ListAgents(context.Background(), tenant.ID("tenant_1"))
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
}

func TestCreateAndEnableAgentFlow(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := New(
		logger,
		infrastructure.NewMemoryAgentRepository(),
		infrastructure.NewMemoryAgentFlowRepository(),
		infrastructure.NewMemoryToolRepository(),
		infrastructure.NewMemorySkillRepository(),
		infrastructure.NewMemoryConnectorRepository(),
		infrastructure.NewMemoryConnectorOperationRepository(),
	)
	ctx := context.Background()
	tenantID := tenant.ID("tenant_1")

	created, err := service.CreateAgentFlow(ctx, agentflow.Spec{
		TenantID: tenantID,
		Name:     "Customer Support Flow",
	})
	if err != nil {
		t.Fatalf("CreateAgentFlow() error = %v", err)
	}
	if created.ID.Empty() {
		t.Fatal("expected generated flow id")
	}
	if created.Status != agentflow.StatusDraft {
		t.Fatalf("status = %s, want draft", created.Status)
	}
	if created.Graph.EntryNodeID != "start" {
		t.Fatalf("entry node = %s, want start", created.Graph.EntryNodeID)
	}
	if created.Graph.Nodes["start"].Type != flowdomain.NodeTypeStart {
		t.Fatalf("start node type = %s, want start", created.Graph.Nodes["start"].Type)
	}

	enabled, err := service.SetAgentFlowStatus(ctx, tenantID, created.ID, agentflow.StatusEnabled)
	if err != nil {
		t.Fatalf("SetAgentFlowStatus() error = %v", err)
	}
	if enabled.Status != agentflow.StatusEnabled || enabled.Graph.Status != agentflow.StatusEnabled {
		t.Fatalf("expected enabled status on spec and graph, got spec=%s graph=%s", enabled.Status, enabled.Graph.Status)
	}

	flows, err := service.ListAgentFlows(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListAgentFlows() error = %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
}

func TestEnablingWorkflowToolAlsoEnablesBoundWorkflow(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	workflowRepo := infrastructure.NewMemoryWorkflowRepository()
	toolRepo := infrastructure.NewMemoryToolRepository()
	service := New(
		logger,
		infrastructure.NewMemoryAgentRepository(),
		infrastructure.NewMemoryAgentFlowRepository(),
		toolRepo,
		infrastructure.NewMemorySkillRepository(),
		infrastructure.NewMemoryConnectorRepository(),
		infrastructure.NewMemoryConnectorOperationRepository(),
		workflowRepo,
	)
	ctx := context.Background()
	tenantID := tenant.ID("tenant_1")
	workflowID := id.ID("wf_doc_upload")
	toolID := id.ID("tool_doc_upload")

	createdWorkflow, err := service.CreateWorkflow(ctx, workflow.Spec{
		ID:       workflowID,
		TenantID: tenantID,
		Name:     "Document Upload Workflow",
		Status:   workflow.StatusDraft,
		Profile:  workflow.ProfileToolWorkflow,
		Graph: workflow.Graph{
			EntryNodeID: id.ID("start"),
			Nodes: map[id.ID]workflow.Node{
				id.ID("start"): {
					ID:   id.ID("start"),
					Type: workflow.NodeTypeStart,
					Name: "Start",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if createdWorkflow.Status != workflow.StatusDraft {
		t.Fatalf("expected draft workflow, got %q", createdWorkflow.Status)
	}

	if _, err := service.CreateTool(ctx, tool.Spec{
		ID:             toolID,
		TenantID:       tenantID,
		Name:           "create_markdown_document",
		Status:         tool.StatusDisabled,
		Implementation: tool.ImplementationWorkflow,
		Binding: tool.Binding{
			WorkflowID: workflowID,
		},
	}); err != nil {
		t.Fatalf("CreateTool() error = %v", err)
	}
	if _, err := service.SetToolStatus(ctx, tenantID, toolID, tool.StatusEnabled); err != nil {
		t.Fatalf("SetToolStatus() error = %v", err)
	}

	enabledWorkflow, err := workflowRepo.GetWorkflow(ctx, tenantID, workflowID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if enabledWorkflow.Status != workflow.StatusEnabled {
		t.Fatalf("expected bound workflow to be enabled, got %q", enabledWorkflow.Status)
	}
}

func TestEnableSupervisorFlowMaterializesLocalSupervisorAgent(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	agentRepo := infrastructure.NewMemoryAgentRepository()
	service := New(
		logger,
		agentRepo,
		infrastructure.NewMemoryAgentFlowRepository(),
		infrastructure.NewMemoryToolRepository(),
		infrastructure.NewMemorySkillRepository(),
		infrastructure.NewMemoryConnectorRepository(),
		infrastructure.NewMemoryConnectorOperationRepository(),
	)
	ctx := context.Background()
	tenantID := tenant.ID("tenant_1")

	created, err := service.CreateAgentFlow(ctx, agentflow.Spec{
		TenantID:          tenantID,
		Name:              "Personal Assistant",
		OrchestrationMode: agentflow.OrchestrationModeSupervisor,
		Graph: flowdomain.FlowGraph{
			EntryNodeID: "start",
			Nodes: map[id.ID]flowdomain.Node{
				"start": {
					ID:   "start",
					Type: flowdomain.NodeTypeStart,
					Name: "Start",
				},
				"supervisor": {
					ID:   "supervisor",
					Type: flowdomain.NodeTypeSupervisor,
					Name: "Local Supervisor",
					Config: map[string]any{
						"agent_mode": "local",
						"local_agent": map[string]any{
							"name":         "Personal Office Assistant",
							"description":  "Route user requests to sub-agents and summarize results.",
							"model":        "deepseek-v4-flash",
							"systemPrompt": "You are a supervisor agent.",
							"skillIds":     []any{},
							"toolIds":      []any{},
						},
					},
				},
				"weather": {
					ID:   "weather",
					Type: flowdomain.NodeTypeAgent,
					Name: "Weather Agent",
					Config: map[string]any{
						"agent_id":   "agent_weather",
						"agent_mode": "existing",
					},
				},
			},
			Edges: []flowdomain.Edge{
				{
					ID:         "start-supervisor",
					FromNodeID: "start",
					ToNodeID:   "supervisor",
					Type:       flowdomain.EdgeTypeDefault,
				},
				{
					ID:         "supervisor-weather",
					FromNodeID: "supervisor",
					ToNodeID:   "weather",
					Type:       flowdomain.EdgeTypeDefault,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateAgentFlow() error = %v", err)
	}
	supervisorAgentID, _ := created.Graph.Nodes["supervisor"].Config["agent_id"].(string)
	if supervisorAgentID == "" {
		t.Fatal("expected local supervisor node to receive a materialized agent_id")
	}
	if created.Supervisor.SupervisorAgentID != id.ID(supervisorAgentID) {
		t.Fatalf("supervisor id = %s, want %s", created.Supervisor.SupervisorAgentID, supervisorAgentID)
	}

	listedAgents, err := service.ListAgents(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(listedAgents) != 0 {
		t.Fatalf("local flow agents should be hidden from ListAgents, got %d", len(listedAgents))
	}

	enabled, err := service.SetAgentFlowStatus(ctx, tenantID, created.ID, agentflow.StatusEnabled)
	if err != nil {
		t.Fatalf("SetAgentFlowStatus() error = %v", err)
	}
	if enabled.Status != agentflow.StatusEnabled {
		t.Fatalf("status = %s, want enabled", enabled.Status)
	}
	hiddenProfile, err := agentRepo.GetAgent(ctx, tenantID, id.ID(supervisorAgentID))
	if err != nil {
		t.Fatalf("GetAgent(local supervisor) error = %v", err)
	}
	if hiddenProfile.SystemPrompt != "You are a supervisor agent." {
		t.Fatalf("system prompt = %q", hiddenProfile.SystemPrompt)
	}
}

func TestCreateSupervisorFlowRejectsExistingAgentWithChildren(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := New(
		logger,
		infrastructure.NewMemoryAgentRepository(),
		infrastructure.NewMemoryAgentFlowRepository(),
		infrastructure.NewMemoryToolRepository(),
		infrastructure.NewMemorySkillRepository(),
		infrastructure.NewMemoryConnectorRepository(),
		infrastructure.NewMemoryConnectorOperationRepository(),
	)
	ctx := context.Background()

	_, err := service.CreateAgentFlow(ctx, agentflow.Spec{
		TenantID:          tenant.ID("tenant_1"),
		Name:              "Invalid Existing Supervisor",
		OrchestrationMode: agentflow.OrchestrationModeSupervisor,
		Graph: flowdomain.FlowGraph{
			EntryNodeID: "start",
			Nodes: map[id.ID]flowdomain.Node{
				"start": {
					ID:   "start",
					Type: flowdomain.NodeTypeStart,
					Name: "Start",
				},
				"supervisor": {
					ID:   "supervisor",
					Type: flowdomain.NodeTypeSupervisor,
					Name: "Existing Supervisor",
					Config: map[string]any{
						"agent_id":   "agent_existing_supervisor",
						"agent_mode": "existing",
					},
				},
				"weather": {
					ID:   "weather",
					Type: flowdomain.NodeTypeAgent,
					Name: "Weather Agent",
					Config: map[string]any{
						"agent_id":   "agent_weather",
						"agent_mode": "existing",
					},
				},
			},
			Edges: []flowdomain.Edge{
				{ID: "start-supervisor", FromNodeID: "start", ToNodeID: "supervisor", Type: flowdomain.EdgeTypeDefault},
				{ID: "supervisor-weather", FromNodeID: "supervisor", ToNodeID: "weather", Type: flowdomain.EdgeTypeDefault},
			},
		},
	})
	if err == nil {
		t.Fatal("expected existing agent with child nodes to be rejected")
	}
	if !strings.Contains(err.Error(), "must be a leaf node") {
		t.Fatalf("expected leaf-node validation error, got %v", err)
	}
}

func TestAgentStatusAndDependencies(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := New(
		logger,
		infrastructure.NewMemoryAgentRepository(),
		infrastructure.NewMemoryAgentFlowRepository(),
		infrastructure.NewMemoryToolRepository(),
		infrastructure.NewMemorySkillRepository(),
		infrastructure.NewMemoryConnectorRepository(),
		infrastructure.NewMemoryConnectorOperationRepository(),
	)
	ctx := context.Background()
	tenantID := tenant.ID("tenant_1")

	createdTool, err := service.CreateTool(ctx, tool.Spec{
		TenantID:       tenantID,
		Name:           "search_help",
		Implementation: tool.ImplementationKnowledge,
		Binding: tool.Binding{
			KnowledgeBaseIDs: []id.ID{id.ID("kb_help")},
		},
	})
	if err != nil {
		t.Fatalf("CreateTool() error = %v", err)
	}
	createdSkill, err := service.CreateSkill(ctx, skill.Spec{
		TenantID: tenantID,
		Name:     "help_center",
		ToolIDs:  []id.ID{createdTool.ID},
	})
	if err != nil {
		t.Fatalf("CreateSkill() error = %v", err)
	}
	createdAgent, err := service.CreateAgent(ctx, agent.Profile{
		TenantID: tenantID,
		Name:     "Help Agent",
		SkillIDs: []id.ID{createdSkill.ID},
	})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	enabled, err := service.SetAgentStatus(ctx, tenantID, createdAgent.ID, agent.StatusEnabled)
	if err != nil {
		t.Fatalf("SetAgentStatus() error = %v", err)
	}
	if enabled.Status != agent.StatusEnabled {
		t.Fatalf("expected enabled agent, got %q", enabled.Status)
	}

	report, err := service.GetAgentDependencies(ctx, tenantID, createdAgent.ID)
	if err != nil {
		t.Fatalf("GetAgentDependencies() error = %v", err)
	}
	if report.Summary.DirectSkillCount != 1 || report.Summary.ReachableToolCount != 1 {
		t.Fatalf("unexpected dependency summary %#v", report.Summary)
	}
}

func TestCreateRegistryResources(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := New(
		logger,
		infrastructure.NewMemoryAgentRepository(),
		infrastructure.NewMemoryAgentFlowRepository(),
		infrastructure.NewMemoryToolRepository(),
		infrastructure.NewMemorySkillRepository(),
		infrastructure.NewMemoryConnectorRepository(),
		infrastructure.NewMemoryConnectorOperationRepository(),
	)
	ctx := context.Background()
	tenantID := tenant.ID("tenant_1")

	operation, err := service.CreateConnectorOperation(ctx, connector.OperationSpec{
		TenantID: tenantID,
		Name:     "query_order",
		BaseURL:  "http://orders.test",
		Method:   "GET",
		Path:     "/orders/{order_id}",
	})
	if err != nil {
		t.Fatalf("CreateConnectorOperation() error = %v", err)
	}
	if operation.ID.Empty() {
		t.Fatal("expected generated connector operation id")
	}
	if operation.Status != connector.OperationStatusDraft {
		t.Fatalf("expected draft connector operation, got %q", operation.Status)
	}
	if operation.BusinessDomain != "General" {
		t.Fatalf("expected default business domain, got %q", operation.BusinessDomain)
	}
	if operation.OwnerTeam != "Integration Team" {
		t.Fatalf("expected default owner team, got %q", operation.OwnerTeam)
	}
	if operation.ImplementationMode != connector.ImplementationModeSimpleHTTP {
		t.Fatalf("expected simple http implementation mode, got %q", operation.ImplementationMode)
	}

	createdTool, err := service.CreateTool(ctx, tool.Spec{
		TenantID:       tenantID,
		Name:           "query_order",
		Implementation: tool.ImplementationConnector,
		Binding: tool.Binding{
			ConnectorOperationID: operation.ID,
		},
	})
	if err != nil {
		t.Fatalf("CreateTool() error = %v", err)
	}
	if createdTool.TimeoutMillis == 0 {
		t.Fatal("expected default timeout")
	}

	createdSkill, err := service.CreateSkill(ctx, skill.Spec{
		TenantID: tenantID,
		Name:     "order_service",
		ToolIDs:  []id.ID{createdTool.ID},
	})
	if err != nil {
		t.Fatalf("CreateSkill() error = %v", err)
	}
	if createdSkill.ID.Empty() {
		t.Fatal("expected generated skill id")
	}

	tools, err := service.ListTools(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	report, err := service.GetConnectorOperationDependencies(ctx, tenantID, operation.ID)
	if err != nil {
		t.Fatalf("GetConnectorOperationDependencies() error = %v", err)
	}
	if report.Summary.DirectToolCount != 1 || report.Summary.IndirectSkillCount != 1 {
		t.Fatalf("unexpected dependency summary %#v", report.Summary)
	}

	enabled, err := service.SetConnectorOperationStatus(ctx, tenantID, operation.ID, connector.OperationStatusEnabled)
	if err != nil {
		t.Fatalf("SetConnectorOperationStatus() error = %v", err)
	}
	if enabled.Status != connector.OperationStatusEnabled {
		t.Fatalf("expected enabled status, got %q", enabled.Status)
	}
}

func TestConnectorOperationStoresOnlyOperationOwnedFields(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	connectorRepo := infrastructure.NewMemoryConnectorRepository()
	operationRepo := infrastructure.NewMemoryConnectorOperationRepository()
	service := New(
		logger,
		infrastructure.NewMemoryAgentRepository(),
		infrastructure.NewMemoryAgentFlowRepository(),
		infrastructure.NewMemoryToolRepository(),
		infrastructure.NewMemorySkillRepository(),
		connectorRepo,
		operationRepo,
	)
	ctx := context.Background()
	tenantID := tenant.ID("tenant_1")

	parent, err := service.CreateConnector(ctx, connector.Spec{
		ID:             id.ID("conn_docs"),
		TenantID:       tenantID,
		Name:           "Google Docs",
		Description:    "Document platform connector",
		BusinessDomain: "Document",
		OwnerTeam:      "Workspace Platform",
		Type:           connector.OperationTypeHTTP,
		Status:         connector.OperationStatusEnabled,
		BaseURL:        "https://docs.example.test",
		Headers:        map[string]string{"X-Workspace": "flow"},
		Auth: connector.AuthConfig{
			Type:      connector.AuthTypeBearer,
			SecretRef: "secret_docs_token",
		},
		TimeoutMillis: 12_000,
	})
	if err != nil {
		t.Fatalf("CreateConnector() error = %v", err)
	}

	operation, err := service.CreateConnectorOperation(ctx, connector.OperationSpec{
		ID:                 id.ID("connop_docs_create"),
		TenantID:           tenantID,
		ConnectorID:        parent.ID,
		Name:               "create_document",
		Description:        "Create a document",
		BusinessDomain:     "Should Be Ignored",
		OwnerTeam:          "Should Be Ignored",
		Type:               connector.OperationTypeHTTP,
		Status:             connector.OperationStatusDraft,
		ImplementationMode: connector.ImplementationModeWorkflow,
		BaseURL:            "https://wrong.example.test",
		Method:             "POST",
		Path:               "/documents",
		Headers:            map[string]string{"X-Operation": "ignored"},
		Auth: connector.AuthConfig{
			Type:      connector.AuthTypeAPIKey,
			SecretRef: "wrong_secret",
		},
		InputSchema:   map[string]any{"type": "object"},
		OutputSchema:  map[string]any{"type": "object"},
		TimeoutMillis: 3_000,
	})
	if err != nil {
		t.Fatalf("CreateConnectorOperation() error = %v", err)
	}
	if operation.BusinessDomain != parent.BusinessDomain || operation.OwnerTeam != parent.OwnerTeam {
		t.Fatalf("expected hydrated parent ownership, got domain=%q owner=%q", operation.BusinessDomain, operation.OwnerTeam)
	}
	if operation.BaseURL != parent.BaseURL {
		t.Fatalf("expected hydrated parent base URL, got %q", operation.BaseURL)
	}
	if operation.Auth.Type != parent.Auth.Type || operation.Auth.SecretRef != parent.Auth.SecretRef {
		t.Fatalf("expected hydrated parent auth, got %#v", operation.Auth)
	}
	if operation.Headers["X-Workspace"] != "flow" {
		t.Fatalf("expected hydrated parent headers, got %#v", operation.Headers)
	}
	if operation.ImplementationMode != connector.ImplementationModeSimpleHTTP {
		t.Fatalf("expected implementation mode to be platform default, got %q", operation.ImplementationMode)
	}

	raw, err := operationRepo.GetConnectorOperation(ctx, tenantID, operation.ID)
	if err != nil {
		t.Fatalf("GetConnectorOperation() raw error = %v", err)
	}
	if raw.BusinessDomain != "" || raw.OwnerTeam != "" {
		t.Fatalf("expected raw operation not to persist connector ownership, got domain=%q owner=%q", raw.BusinessDomain, raw.OwnerTeam)
	}
	if raw.BaseURL != "" {
		t.Fatalf("expected raw operation not to persist connector base URL, got %q", raw.BaseURL)
	}
	if len(raw.Headers) != 0 {
		t.Fatalf("expected raw operation not to persist connector headers, got %#v", raw.Headers)
	}
	if raw.Auth.Type != "" || raw.Auth.SecretRef != "" {
		t.Fatalf("expected raw operation not to persist connector auth, got %#v", raw.Auth)
	}
}

func TestImportConnectorToolsIsIdempotentByOperation(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := New(
		logger,
		infrastructure.NewMemoryAgentRepository(),
		infrastructure.NewMemoryAgentFlowRepository(),
		infrastructure.NewMemoryToolRepository(),
		infrastructure.NewMemorySkillRepository(),
		infrastructure.NewMemoryConnectorRepository(),
		infrastructure.NewMemoryConnectorOperationRepository(),
	)
	ctx := context.Background()
	tenantID := tenant.ID("tenant_1")

	parent, err := service.CreateConnector(ctx, connector.Spec{
		ID:             id.ID("conn_weather"),
		TenantID:       tenantID,
		Name:           "Weather API",
		BusinessDomain: "Weather",
		OwnerTeam:      "AI Platform",
		Type:           connector.OperationTypeHTTP,
		BaseURL:        "https://weather.example.test",
		TimeoutMillis:  8000,
	})
	if err != nil {
		t.Fatalf("CreateConnector() error = %v", err)
	}
	query, err := service.CreateConnectorOperation(ctx, connector.OperationSpec{
		ID:          id.ID("connop_weather_query"),
		TenantID:    tenantID,
		ConnectorID: parent.ID,
		Name:        "query_weather",
		Description: "Query current weather",
		Type:        connector.OperationTypeHTTP,
		Method:      "GET",
		Path:        "/weather/current",
		InputSchema: map[string]any{
			"type":       "object",
			"required":   []string{"city"},
			"properties": map[string]any{"city": map[string]any{"type": "string"}},
		},
		OutputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"condition": map[string]any{"type": "string"}},
		},
		TimeoutMillis: 5000,
	})
	if err != nil {
		t.Fatalf("CreateConnectorOperation(query) error = %v", err)
	}
	create, err := service.CreateConnectorOperation(ctx, connector.OperationSpec{
		ID:            id.ID("connop_weather_alert"),
		TenantID:      tenantID,
		ConnectorID:   parent.ID,
		Name:          "create_weather_alert",
		Description:   "Create a weather alert",
		Type:          connector.OperationTypeHTTP,
		Method:        "POST",
		Path:          "/weather/alerts",
		InputSchema:   map[string]any{"type": "object"},
		OutputSchema:  map[string]any{"type": "object"},
		TimeoutMillis: 6000,
	})
	if err != nil {
		t.Fatalf("CreateConnectorOperation(create) error = %v", err)
	}

	imported, err := service.ImportConnectorTools(ctx, tenantID, parent.ID, []tool.Spec{
		{
			Name:           query.Name,
			Description:    query.Description,
			Implementation: tool.ImplementationConnector,
			Binding: tool.Binding{
				ConnectorOperationID: query.ID,
			},
			InputSchema:   query.InputSchema,
			OutputSchema:  query.OutputSchema,
			SideEffect:    tool.SideEffectRead,
			RiskLevel:     tool.RiskLow,
			TimeoutMillis: 5000,
			Status:        tool.StatusDisabled,
		},
		{
			Name:           create.Name,
			Description:    create.Description,
			Implementation: tool.ImplementationConnector,
			Binding: tool.Binding{
				ConnectorOperationID: create.ID,
			},
			InputSchema:          create.InputSchema,
			OutputSchema:         create.OutputSchema,
			SideEffect:           tool.SideEffectWrite,
			RiskLevel:            tool.RiskMedium,
			RequiresConfirmation: true,
			TimeoutMillis:        6000,
			Status:               tool.StatusDisabled,
		},
	})
	if err != nil {
		t.Fatalf("ImportConnectorTools() error = %v", err)
	}
	if len(imported) != 2 {
		t.Fatalf("expected 2 imported tools, got %d", len(imported))
	}
	if imported[0].BusinessDomain != parent.BusinessDomain || imported[0].OwnerTeam != parent.OwnerTeam {
		t.Fatalf("expected connector ownership defaults, got domain=%q owner=%q", imported[0].BusinessDomain, imported[0].OwnerTeam)
	}
	firstImportIDs := map[id.ID]struct{}{}
	for _, spec := range imported {
		firstImportIDs[spec.ID] = struct{}{}
	}

	importedAgain, err := service.ImportConnectorTools(ctx, tenantID, parent.ID, []tool.Spec{
		{
			Name:           "query_weather_v2",
			Description:    query.Description,
			Implementation: tool.ImplementationConnector,
			Binding: tool.Binding{
				ConnectorOperationID: query.ID,
			},
			InputSchema:   query.InputSchema,
			OutputSchema:  query.OutputSchema,
			SideEffect:    tool.SideEffectRead,
			RiskLevel:     tool.RiskLow,
			TimeoutMillis: 5000,
			Status:        tool.StatusDisabled,
		},
	})
	if err != nil {
		t.Fatalf("ImportConnectorTools(second) error = %v", err)
	}
	if len(importedAgain) != 1 {
		t.Fatalf("expected 1 re-imported tool, got %d", len(importedAgain))
	}
	if _, ok := firstImportIDs[importedAgain[0].ID]; !ok {
		t.Fatalf("expected re-import to update existing tool id, got %q", importedAgain[0].ID)
	}

	allTools, err := service.ListTools(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(allTools) != 2 {
		t.Fatalf("expected idempotent import to keep 2 tools, got %d", len(allTools))
	}
}
