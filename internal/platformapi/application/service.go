package application

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	flowdomain "flow-anything/internal/agentflow/domain"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
	"flow-anything/internal/platformapi/domain"
	"flow-anything/internal/platformapi/ports"
)

type Service struct {
	logger     *slog.Logger
	agents     ports.AgentRepository
	agentFlows ports.AgentFlowRepository
	tools      ports.ToolRepository
	skills     ports.SkillRepository
	connectors ports.ConnectorRepository
	operations ports.ConnectorOperationRepository
	workflows  ports.WorkflowRepository
}

const flowLocalAgentOwnerTeam = "Agent Flow Local"

func fixedAgentFlowInputSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Agent Flow user-facing input contract.",
		"properties": map[string]any{
			"user_request": map[string]any{
				"type":        "string",
				"description": "The raw request text submitted by the user.",
			},
		},
		"required": []any{"user_request"},
		"x-flow-fields": []any{
			map[string]any{
				"path":        "user_request",
				"type":        "string",
				"description": "The raw request text submitted by the user.",
				"required":    true,
			},
		},
	}
}

func fixedAgentFlowOutputSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Agent Flow user-facing output contract.",
		"properties": map[string]any{
			"return_message": map[string]any{
				"type":        "string",
				"description": "The final message returned to the user.",
			},
		},
		"required": []any{"return_message"},
		"x-flow-fields": []any{
			map[string]any{
				"path":        "return_message",
				"type":        "string",
				"description": "The final message returned to the user.",
				"required":    true,
			},
		},
	}
}

func New(
	logger *slog.Logger,
	agents ports.AgentRepository,
	agentFlows ports.AgentFlowRepository,
	tools ports.ToolRepository,
	skills ports.SkillRepository,
	connectors ports.ConnectorRepository,
	operations ports.ConnectorOperationRepository,
	workflowRepos ...ports.WorkflowRepository,
) *Service {
	var workflows ports.WorkflowRepository
	if len(workflowRepos) > 0 {
		workflows = workflowRepos[0]
	}
	return &Service{
		logger:     logger,
		agents:     agents,
		agentFlows: agentFlows,
		tools:      tools,
		skills:     skills,
		connectors: connectors,
		operations: operations,
		workflows:  workflows,
	}
}

func (s *Service) CreateAgent(ctx context.Context, profile agent.Profile) (agent.Profile, error) {
	if profile.ID.Empty() {
		profile.ID = id.New("agent")
	}
	if profile.Version == "" {
		profile.Version = "v1"
	}
	applyAgentDefaults(&profile, nil)
	if err := domain.ValidateAgent(profile); err != nil {
		return agent.Profile{}, err
	}

	if err := s.agents.SaveAgent(ctx, profile); err != nil {
		return agent.Profile{}, err
	}

	s.logger.Info("agent created", "agent_id", profile.ID.String())
	return profile, nil
}

func (s *Service) UpdateAgent(ctx context.Context, profile agent.Profile) (agent.Profile, error) {
	if profile.TenantID.Empty() {
		return agent.Profile{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if profile.ID.Empty() {
		return agent.Profile{}, apperrors.New(apperrors.CodeInvalidArgument, "agent_id is required")
	}

	current, err := s.agents.GetAgent(ctx, profile.TenantID, profile.ID)
	if err != nil {
		return agent.Profile{}, err
	}
	if profile.Version == "" {
		profile.Version = current.Version
	}
	applyAgentDefaults(&profile, &current)
	if err := domain.ValidateAgent(profile); err != nil {
		return agent.Profile{}, err
	}
	if err := s.agents.SaveAgent(ctx, profile); err != nil {
		return agent.Profile{}, err
	}

	s.logger.Info("agent updated", "agent_id", profile.ID.String(), "status", profile.Status)
	return profile, nil
}

func (s *Service) SetAgentStatus(ctx context.Context, tenantID tenant.ID, agentID id.ID, status agent.Status) (agent.Profile, error) {
	if tenantID.Empty() {
		return agent.Profile{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if agentID.Empty() {
		return agent.Profile{}, apperrors.New(apperrors.CodeInvalidArgument, "agent_id is required")
	}
	if status != agent.StatusEnabled && status != agent.StatusDisabled {
		return agent.Profile{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported agent status transition")
	}

	profile, err := s.agents.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return agent.Profile{}, err
	}
	profile.Status = status
	if err := s.agents.SaveAgent(ctx, profile); err != nil {
		return agent.Profile{}, err
	}

	s.logger.Info("agent status changed", "agent_id", agentID.String(), "status", status)
	return profile, nil
}

func (s *Service) ListAgents(ctx context.Context, tenantID tenant.ID) ([]agent.Profile, error) {
	profiles, err := s.agents.ListAgents(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]agent.Profile, 0, len(profiles))
	for _, profile := range profiles {
		if isFlowLocalAgent(profile) {
			continue
		}
		result = append(result, profile)
	}
	return result, nil
}

func (s *Service) GetAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Profile, error) {
	return s.agents.GetAgent(ctx, tenantID, agentID)
}

func (s *Service) GetAgentDependencies(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Dependencies, error) {
	profile, err := s.agents.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return agent.Dependencies{}, err
	}

	skills, err := s.skills.ListSkills(ctx, tenantID)
	if err != nil {
		return agent.Dependencies{}, err
	}
	tools, err := s.tools.ListTools(ctx, tenantID)
	if err != nil {
		return agent.Dependencies{}, err
	}

	skillByID := make(map[id.ID]skill.Spec, len(skills))
	for _, spec := range skills {
		skillByID[spec.ID] = spec
	}
	toolByID := make(map[id.ID]tool.Spec, len(tools))
	for _, spec := range tools {
		toolByID[spec.ID] = spec
	}

	report := agent.Dependencies{
		AgentID:        agentID,
		DirectSkills:   []agent.SkillBinding{},
		ReachableTools: []agent.ToolBinding{},
	}
	seenToolIDs := map[id.ID]struct{}{}
	addReachableTool := func(toolID id.ID, viaSkillID id.ID, source string) bool {
		if _, ok := seenToolIDs[toolID]; ok {
			return false
		}
		toolSpec, ok := toolByID[toolID]
		if !ok {
			return false
		}
		seenToolIDs[toolID] = struct{}{}
		report.ReachableTools = append(report.ReachableTools, agent.ToolBinding{
			ID:             toolSpec.ID,
			Name:           toolSpec.Name,
			ViaSkillID:     viaSkillID,
			Source:         source,
			Implementation: toolSpec.Implementation,
			RiskLevel:      toolSpec.RiskLevel,
			Status:         string(toolSpec.Status),
		})
		return true
	}

	for _, toolID := range profile.ToolIDs {
		if addReachableTool(toolID, "", "direct") {
			report.Summary.DirectToolCount++
		}
	}

	for _, skillID := range profile.SkillIDs {
		spec, ok := skillByID[skillID]
		if !ok {
			continue
		}
		report.DirectSkills = append(report.DirectSkills, agent.SkillBinding{
			ID:     spec.ID,
			Name:   spec.Name,
			Status: string(spec.Status),
		})
		if spec.Status == skill.StatusDisabled || spec.Status == skill.StatusDraft {
			report.Summary.DisabledSkillCount++
		}
		for _, toolID := range spec.ToolIDs {
			addReachableTool(toolID, spec.ID, "skill")
		}
	}

	report.Summary.DirectSkillCount = len(report.DirectSkills)
	report.Summary.ReachableToolCount = len(report.ReachableTools)
	report.Summary.TotalCapabilityCount = report.Summary.DirectSkillCount + report.Summary.ReachableToolCount
	return report, nil
}

func (s *Service) CreateAgentFlow(ctx context.Context, spec agentflow.Spec) (agentflow.Spec, error) {
	if s.agentFlows == nil {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "agent flow repository is not configured")
	}
	if spec.ID.Empty() {
		spec.ID = id.New("flow")
	}
	if spec.Version == "" {
		spec.Version = "v1"
	}
	applyAgentFlowDefaults(&spec, nil)
	if err := s.materializeLocalAgentFlowNodes(ctx, &spec); err != nil {
		return agentflow.Spec{}, err
	}
	normalizeSupervisorFromGraph(&spec)
	if err := domain.ValidateAgentFlow(spec); err != nil {
		return agentflow.Spec{}, err
	}
	if err := s.agentFlows.SaveAgentFlow(ctx, spec); err != nil {
		return agentflow.Spec{}, err
	}

	s.logger.Info("agent flow created", "flow_id", spec.ID.String())
	return spec, nil
}

func (s *Service) UpdateAgentFlow(ctx context.Context, spec agentflow.Spec) (agentflow.Spec, error) {
	if s.agentFlows == nil {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "agent flow repository is not configured")
	}
	if spec.TenantID.Empty() {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if spec.ID.Empty() {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "flow_id is required")
	}

	current, err := s.agentFlows.GetAgentFlow(ctx, spec.TenantID, spec.ID)
	if err != nil {
		return agentflow.Spec{}, err
	}
	if spec.Version == "" {
		spec.Version = current.Version
	}
	applyAgentFlowDefaults(&spec, &current)
	if err := s.materializeLocalAgentFlowNodes(ctx, &spec); err != nil {
		return agentflow.Spec{}, err
	}
	normalizeSupervisorFromGraph(&spec)
	if err := domain.ValidateAgentFlow(spec); err != nil {
		return agentflow.Spec{}, err
	}
	if err := s.agentFlows.SaveAgentFlow(ctx, spec); err != nil {
		return agentflow.Spec{}, err
	}

	s.logger.Info("agent flow updated", "flow_id", spec.ID.String(), "status", spec.Status)
	return spec, nil
}

func (s *Service) SetAgentFlowStatus(ctx context.Context, tenantID tenant.ID, flowID id.ID, status agentflow.Status) (agentflow.Spec, error) {
	if s.agentFlows == nil {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "agent flow repository is not configured")
	}
	if tenantID.Empty() {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if flowID.Empty() {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "flow_id is required")
	}
	if status != agentflow.StatusEnabled && status != agentflow.StatusDisabled {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported agent flow status transition")
	}

	spec, err := s.agentFlows.GetAgentFlow(ctx, tenantID, flowID)
	if err != nil {
		return agentflow.Spec{}, err
	}
	spec.Status = status
	spec.Graph.Status = status
	applyAgentFlowDefaults(&spec, nil)
	if err := s.materializeLocalAgentFlowNodes(ctx, &spec); err != nil {
		return agentflow.Spec{}, err
	}
	normalizeSupervisorFromGraph(&spec)
	if err := domain.ValidateAgentFlow(spec); err != nil {
		return agentflow.Spec{}, err
	}
	if err := s.agentFlows.SaveAgentFlow(ctx, spec); err != nil {
		return agentflow.Spec{}, err
	}

	s.logger.Info("agent flow status changed", "flow_id", flowID.String(), "status", status)
	return spec, nil
}

func (s *Service) ListAgentFlows(ctx context.Context, tenantID tenant.ID) ([]agentflow.Spec, error) {
	if s.agentFlows == nil {
		return []agentflow.Spec{}, nil
	}
	return s.agentFlows.ListAgentFlows(ctx, tenantID)
}

func (s *Service) GetAgentFlow(ctx context.Context, tenantID tenant.ID, flowID id.ID) (agentflow.Spec, error) {
	if s.agentFlows == nil {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "agent flow repository is not configured")
	}
	return s.agentFlows.GetAgentFlow(ctx, tenantID, flowID)
}

func (s *Service) CreateWorkflow(ctx context.Context, spec workflow.Spec) (workflow.Spec, error) {
	if s.workflows == nil {
		return workflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "workflow repository is not configured")
	}
	if spec.ID.Empty() {
		spec.ID = id.New("wf")
	}
	if spec.Version == "" {
		spec.Version = "v1"
	}
	applyWorkflowDefaults(&spec, nil)
	if err := domain.ValidateWorkflow(spec); err != nil {
		return workflow.Spec{}, err
	}
	if err := s.workflows.SaveWorkflow(ctx, spec); err != nil {
		return workflow.Spec{}, err
	}

	s.logger.Info("workflow created", "workflow_id", spec.ID.String(), "profile", spec.Profile)
	return spec, nil
}

func (s *Service) UpdateWorkflow(ctx context.Context, spec workflow.Spec) (workflow.Spec, error) {
	if s.workflows == nil {
		return workflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "workflow repository is not configured")
	}
	if spec.TenantID.Empty() {
		return workflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if spec.ID.Empty() {
		return workflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "workflow_id is required")
	}

	current, err := s.workflows.GetWorkflow(ctx, spec.TenantID, spec.ID)
	if err != nil {
		return workflow.Spec{}, err
	}
	if spec.Version == "" {
		spec.Version = current.Version
	}
	applyWorkflowDefaults(&spec, &current)
	if err := domain.ValidateWorkflow(spec); err != nil {
		return workflow.Spec{}, err
	}
	if err := s.workflows.SaveWorkflow(ctx, spec); err != nil {
		return workflow.Spec{}, err
	}

	s.logger.Info("workflow updated", "workflow_id", spec.ID.String(), "status", spec.Status)
	return spec, nil
}

func (s *Service) SetWorkflowStatus(ctx context.Context, tenantID tenant.ID, workflowID id.ID, status workflow.Status) (workflow.Spec, error) {
	if s.workflows == nil {
		return workflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "workflow repository is not configured")
	}
	if tenantID.Empty() {
		return workflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if workflowID.Empty() {
		return workflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "workflow_id is required")
	}
	if status != workflow.StatusEnabled && status != workflow.StatusDisabled {
		return workflow.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported workflow status transition")
	}

	spec, err := s.workflows.GetWorkflow(ctx, tenantID, workflowID)
	if err != nil {
		return workflow.Spec{}, err
	}
	spec.Status = status
	applyWorkflowDefaults(&spec, nil)
	if err := domain.ValidateWorkflow(spec); err != nil {
		return workflow.Spec{}, err
	}
	if status == workflow.StatusEnabled {
		if err := s.validateWorkflowForPublish(ctx, spec); err != nil {
			return workflow.Spec{}, err
		}
	}
	if err := s.workflows.SaveWorkflow(ctx, spec); err != nil {
		return workflow.Spec{}, err
	}

	s.logger.Info("workflow status changed", "workflow_id", workflowID.String(), "status", status)
	return spec, nil
}

func (s *Service) ListWorkflows(ctx context.Context, tenantID tenant.ID) ([]workflow.Spec, error) {
	if s.workflows == nil {
		return []workflow.Spec{}, nil
	}
	return s.workflows.ListWorkflows(ctx, tenantID)
}

func (s *Service) GetWorkflow(ctx context.Context, tenantID tenant.ID, workflowID id.ID) (workflow.Spec, error) {
	if s.workflows == nil {
		return workflow.Spec{}, apperrors.New(apperrors.CodeUnavailable, "workflow repository is not configured")
	}
	return s.workflows.GetWorkflow(ctx, tenantID, workflowID)
}

func (s *Service) CreateTool(ctx context.Context, spec tool.Spec) (tool.Spec, error) {
	if spec.ID.Empty() {
		spec.ID = id.New("tool")
	}
	if spec.Version == "" {
		spec.Version = "v1"
	}
	applyToolDefaults(&spec, nil)
	if err := domain.ValidateTool(spec); err != nil {
		return tool.Spec{}, err
	}
	if err := s.ensureBoundWorkflowEnabled(ctx, spec); err != nil {
		return tool.Spec{}, err
	}

	if err := s.tools.SaveTool(ctx, spec); err != nil {
		return tool.Spec{}, err
	}

	s.logger.Info("tool created", "tool_id", spec.ID.String(), "implementation", spec.Implementation)
	return spec, nil
}

func (s *Service) UpdateTool(ctx context.Context, spec tool.Spec) (tool.Spec, error) {
	if spec.TenantID.Empty() {
		return tool.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if spec.ID.Empty() {
		return tool.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tool_id is required")
	}

	current, err := s.tools.GetTool(ctx, spec.TenantID, spec.ID)
	if err != nil {
		return tool.Spec{}, err
	}
	if spec.Version == "" {
		spec.Version = current.Version
	}
	applyToolDefaults(&spec, &current)
	if err := domain.ValidateTool(spec); err != nil {
		return tool.Spec{}, err
	}
	if err := s.ensureBoundWorkflowEnabled(ctx, spec); err != nil {
		return tool.Spec{}, err
	}
	if err := s.tools.SaveTool(ctx, spec); err != nil {
		return tool.Spec{}, err
	}

	s.logger.Info("tool updated", "tool_id", spec.ID.String(), "status", spec.Status)
	return spec, nil
}

// ImportConnectorTools materializes one platform Tool per Connector Operation.
// It is idempotent by ConnectorOperationID: importing the same Connector again
// updates existing tools instead of creating duplicates.
func (s *Service) ImportConnectorTools(ctx context.Context, tenantID tenant.ID, connectorID id.ID, specs []tool.Spec) ([]tool.Spec, error) {
	if tenantID.Empty() {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if connectorID.Empty() {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "connector_id is required")
	}
	if s.connectors == nil {
		return nil, apperrors.New(apperrors.CodeUnavailable, "connector repository is not configured")
	}

	connectorSpec, err := s.connectors.GetConnector(ctx, tenantID, connectorID)
	if err != nil {
		return nil, err
	}
	operations, err := s.operations.ListConnectorOperations(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	operationsByID := map[id.ID]connector.OperationSpec{}
	for _, operation := range operations {
		if operation.ConnectorID != connectorID {
			continue
		}
		operationsByID[operation.ID] = operation
	}
	if len(operationsByID) == 0 {
		return []tool.Spec{}, nil
	}

	if len(specs) == 0 {
		specs = make([]tool.Spec, 0, len(operationsByID))
		for _, operation := range operationsByID {
			specs = append(specs, newToolSpecFromConnectorOperation(connectorSpec, operation))
		}
	}

	existingByOperationID, err := s.connectorToolsByOperationID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	imported := make([]tool.Spec, 0, len(specs))
	for _, spec := range specs {
		spec.TenantID = tenantID
		spec.Implementation = tool.ImplementationConnector
		operationID := spec.Binding.ConnectorOperationID
		operation, ok := operationsByID[operationID]
		if !ok {
			return nil, apperrors.New(apperrors.CodeInvalidArgument, "connector_operation_id does not belong to connector")
		}

		if existing, ok := existingByOperationID[operationID]; ok && spec.ID.Empty() {
			spec.ID = existing.ID
			if spec.Version == "" {
				spec.Version = existing.Version
			}
		}
		applyConnectorToolDefaults(&spec, connectorSpec, operation)
		if err := domain.ValidateTool(spec); err != nil {
			return nil, err
		}
		if err := s.tools.SaveTool(ctx, spec); err != nil {
			return nil, err
		}
		imported = append(imported, spec)
	}

	s.logger.Info("connector tools imported", "connector_id", connectorID.String(), "tool_count", len(imported))
	return imported, nil
}

func (s *Service) SetToolStatus(ctx context.Context, tenantID tenant.ID, toolID id.ID, status tool.Status) (tool.Spec, error) {
	if tenantID.Empty() {
		return tool.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if toolID.Empty() {
		return tool.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tool_id is required")
	}
	if status != tool.StatusEnabled && status != tool.StatusDisabled {
		return tool.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported tool status transition")
	}

	spec, err := s.tools.GetTool(ctx, tenantID, toolID)
	if err != nil {
		return tool.Spec{}, err
	}
	spec.Status = status
	if err := s.ensureBoundWorkflowEnabled(ctx, spec); err != nil {
		return tool.Spec{}, err
	}
	if err := s.tools.SaveTool(ctx, spec); err != nil {
		return tool.Spec{}, err
	}

	s.logger.Info("tool status changed", "tool_id", toolID.String(), "status", status)
	return spec, nil
}

func (s *Service) ensureBoundWorkflowEnabled(ctx context.Context, spec tool.Spec) error {
	if spec.Status != tool.StatusEnabled ||
		spec.Implementation != tool.ImplementationWorkflow ||
		spec.Binding.WorkflowID.Empty() ||
		s.workflows == nil {
		return nil
	}

	workflowSpec, err := s.workflows.GetWorkflow(ctx, spec.TenantID, spec.Binding.WorkflowID)
	if err != nil {
		return err
	}
	if workflowSpec.Status == workflow.StatusEnabled {
		return nil
	}

	workflowSpec.Status = workflow.StatusEnabled
	applyWorkflowDefaults(&workflowSpec, nil)
	if err := domain.ValidateWorkflow(workflowSpec); err != nil {
		return err
	}
	if err := s.workflows.SaveWorkflow(ctx, workflowSpec); err != nil {
		return err
	}

	s.logger.Info(
		"workflow status synced from enabled workflow tool",
		"tool_id", spec.ID.String(),
		"workflow_id", workflowSpec.ID.String(),
		"status", workflowSpec.Status,
	)
	return nil
}

func (s *Service) ListTools(ctx context.Context, tenantID tenant.ID) ([]tool.Spec, error) {
	return s.tools.ListTools(ctx, tenantID)
}

func (s *Service) GetTool(ctx context.Context, tenantID tenant.ID, toolID id.ID) (tool.Spec, error) {
	return s.tools.GetTool(ctx, tenantID, toolID)
}

func (s *Service) GetToolDependencies(ctx context.Context, tenantID tenant.ID, toolID id.ID) (tool.Dependencies, error) {
	if _, err := s.tools.GetTool(ctx, tenantID, toolID); err != nil {
		return tool.Dependencies{}, err
	}

	skills, err := s.skills.ListSkills(ctx, tenantID)
	if err != nil {
		return tool.Dependencies{}, err
	}
	agents, err := s.agents.ListAgents(ctx, tenantID)
	if err != nil {
		return tool.Dependencies{}, err
	}

	report := tool.Dependencies{
		ToolID:         toolID,
		DirectSkills:   []tool.SkillConsumer{},
		DirectAgents:   []tool.AgentConsumer{},
		IndirectAgents: []tool.AgentConsumer{},
	}

	skillIDs := map[id.ID]struct{}{}
	for _, spec := range skills {
		for _, currentToolID := range spec.ToolIDs {
			if currentToolID != toolID {
				continue
			}
			skillIDs[spec.ID] = struct{}{}
			report.DirectSkills = append(report.DirectSkills, tool.SkillConsumer{
				ID:   spec.ID,
				Name: spec.Name,
			})
			break
		}
	}

	for _, profile := range agents {
		direct := false
		for _, currentToolID := range profile.ToolIDs {
			if currentToolID != toolID {
				continue
			}
			direct = true
			report.DirectAgents = append(report.DirectAgents, tool.AgentConsumer{
				ID:     profile.ID,
				Name:   profile.Name,
				Source: "direct",
			})
			break
		}
		if direct {
			continue
		}
		for _, skillID := range profile.SkillIDs {
			if _, ok := skillIDs[skillID]; !ok {
				continue
			}
			report.IndirectAgents = append(report.IndirectAgents, tool.AgentConsumer{
				ID:         profile.ID,
				Name:       profile.Name,
				ViaSkillID: skillID,
				Source:     "skill",
			})
			break
		}
	}

	report.Summary.DirectSkillCount = len(report.DirectSkills)
	report.Summary.DirectAgentCount = len(report.DirectAgents)
	report.Summary.IndirectAgentCount = len(report.IndirectAgents)
	report.Summary.TotalConsumerCount = report.Summary.DirectSkillCount + report.Summary.DirectAgentCount + report.Summary.IndirectAgentCount
	return report, nil
}

func (s *Service) CreateSkill(ctx context.Context, spec skill.Spec) (skill.Spec, error) {
	if spec.ID.Empty() {
		spec.ID = id.New("skill")
	}
	if spec.Version == "" {
		spec.Version = "v1"
	}
	applySkillDefaults(&spec, nil)
	if err := domain.ValidateSkill(spec); err != nil {
		return skill.Spec{}, err
	}

	if err := s.skills.SaveSkill(ctx, spec); err != nil {
		return skill.Spec{}, err
	}

	s.logger.Info("skill created", "skill_id", spec.ID.String())
	return spec, nil
}

func (s *Service) UpdateSkill(ctx context.Context, spec skill.Spec) (skill.Spec, error) {
	if spec.TenantID.Empty() {
		return skill.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if spec.ID.Empty() {
		return skill.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "skill_id is required")
	}

	current, err := s.skills.GetSkill(ctx, spec.TenantID, spec.ID)
	if err != nil {
		return skill.Spec{}, err
	}
	if spec.Version == "" {
		spec.Version = current.Version
	}
	applySkillDefaults(&spec, &current)
	if err := domain.ValidateSkill(spec); err != nil {
		return skill.Spec{}, err
	}
	if err := s.skills.SaveSkill(ctx, spec); err != nil {
		return skill.Spec{}, err
	}

	s.logger.Info("skill updated", "skill_id", spec.ID.String(), "status", spec.Status)
	return spec, nil
}

func (s *Service) SetSkillStatus(ctx context.Context, tenantID tenant.ID, skillID id.ID, status skill.Status) (skill.Spec, error) {
	if tenantID.Empty() {
		return skill.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if skillID.Empty() {
		return skill.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "skill_id is required")
	}
	if status != skill.StatusEnabled && status != skill.StatusDisabled {
		return skill.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported skill status transition")
	}

	spec, err := s.skills.GetSkill(ctx, tenantID, skillID)
	if err != nil {
		return skill.Spec{}, err
	}
	spec.Status = status
	if err := s.skills.SaveSkill(ctx, spec); err != nil {
		return skill.Spec{}, err
	}

	s.logger.Info("skill status changed", "skill_id", skillID.String(), "status", status)
	return spec, nil
}

func (s *Service) ListSkills(ctx context.Context, tenantID tenant.ID) ([]skill.Spec, error) {
	return s.skills.ListSkills(ctx, tenantID)
}

func (s *Service) GetSkill(ctx context.Context, tenantID tenant.ID, skillID id.ID) (skill.Spec, error) {
	return s.skills.GetSkill(ctx, tenantID, skillID)
}

func (s *Service) GetSkillDependencies(ctx context.Context, tenantID tenant.ID, skillID id.ID) (skill.Dependencies, error) {
	if _, err := s.skills.GetSkill(ctx, tenantID, skillID); err != nil {
		return skill.Dependencies{}, err
	}

	agents, err := s.agents.ListAgents(ctx, tenantID)
	if err != nil {
		return skill.Dependencies{}, err
	}

	report := skill.Dependencies{
		SkillID:      skillID,
		DirectAgents: []skill.AgentConsumer{},
	}
	for _, profile := range agents {
		for _, currentSkillID := range profile.SkillIDs {
			if currentSkillID != skillID {
				continue
			}
			report.DirectAgents = append(report.DirectAgents, skill.AgentConsumer{
				ID:   profile.ID,
				Name: profile.Name,
			})
			break
		}
	}

	report.Summary.DirectAgentCount = len(report.DirectAgents)
	report.Summary.TotalConsumerCount = report.Summary.DirectAgentCount
	return report, nil
}

func (s *Service) CreateConnector(ctx context.Context, spec connector.Spec) (connector.Spec, error) {
	if s.connectors == nil {
		return connector.Spec{}, apperrors.New(apperrors.CodeUnavailable, "connector repository is not configured")
	}
	if spec.ID.Empty() {
		spec.ID = id.New("conn")
	}
	if spec.Type == "" {
		spec.Type = connector.OperationTypeHTTP
	}
	if spec.Status == "" {
		spec.Status = connector.OperationStatusDraft
	}
	if spec.Version == "" {
		spec.Version = "v1"
	}
	applyConnectorDefaults(&spec, nil)
	if err := domain.ValidateConnector(spec); err != nil {
		return connector.Spec{}, err
	}
	if err := s.connectors.SaveConnector(ctx, spec); err != nil {
		return connector.Spec{}, err
	}

	s.logger.Info("connector created", "connector_id", spec.ID.String())
	return spec, nil
}

func (s *Service) UpdateConnector(ctx context.Context, spec connector.Spec) (connector.Spec, error) {
	if s.connectors == nil {
		return connector.Spec{}, apperrors.New(apperrors.CodeUnavailable, "connector repository is not configured")
	}
	if spec.TenantID.Empty() {
		return connector.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if spec.ID.Empty() {
		return connector.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "connector_id is required")
	}
	current, err := s.connectors.GetConnector(ctx, spec.TenantID, spec.ID)
	if err != nil {
		return connector.Spec{}, err
	}
	if spec.Type == "" {
		spec.Type = connector.OperationTypeHTTP
	}
	if spec.Status == "" {
		spec.Status = current.Status
	}
	if spec.Version == "" {
		spec.Version = current.Version
	}
	applyConnectorDefaults(&spec, &current)
	if err := domain.ValidateConnector(spec); err != nil {
		return connector.Spec{}, err
	}
	if err := s.connectors.SaveConnector(ctx, spec); err != nil {
		return connector.Spec{}, err
	}

	s.logger.Info("connector updated", "connector_id", spec.ID.String(), "status", spec.Status)
	return spec, nil
}

func (s *Service) SetConnectorStatus(ctx context.Context, tenantID tenant.ID, connectorID id.ID, status connector.OperationStatus) (connector.Spec, error) {
	if s.connectors == nil {
		return connector.Spec{}, apperrors.New(apperrors.CodeUnavailable, "connector repository is not configured")
	}
	if tenantID.Empty() {
		return connector.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if connectorID.Empty() {
		return connector.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "connector_id is required")
	}
	if status != connector.OperationStatusEnabled && status != connector.OperationStatusDisabled {
		return connector.Spec{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector status transition")
	}

	spec, err := s.connectors.GetConnector(ctx, tenantID, connectorID)
	if err != nil {
		return connector.Spec{}, err
	}
	spec.Status = status
	if err := s.connectors.SaveConnector(ctx, spec); err != nil {
		return connector.Spec{}, err
	}

	s.logger.Info("connector status changed", "connector_id", connectorID.String(), "status", status)
	return spec, nil
}

func (s *Service) ListConnectors(ctx context.Context, tenantID tenant.ID) ([]connector.Spec, error) {
	if s.connectors == nil {
		return []connector.Spec{}, nil
	}
	return s.connectors.ListConnectors(ctx, tenantID)
}

func (s *Service) GetConnector(ctx context.Context, tenantID tenant.ID, connectorID id.ID) (connector.Spec, error) {
	if s.connectors == nil {
		return connector.Spec{}, apperrors.New(apperrors.CodeUnavailable, "connector repository is not configured")
	}
	return s.connectors.GetConnector(ctx, tenantID, connectorID)
}

func (s *Service) CreateConnectorOperation(ctx context.Context, spec connector.OperationSpec) (connector.OperationSpec, error) {
	if spec.ID.Empty() {
		spec.ID = id.New("connop")
	}
	if spec.Type == "" {
		spec.Type = connector.OperationTypeHTTP
	}
	if spec.Status == "" {
		spec.Status = connector.OperationStatusDraft
	}
	applyConnectorOperationDefaults(&spec, nil)
	if err := s.normalizeConnectorOperationForStorage(ctx, &spec); err != nil {
		return connector.OperationSpec{}, err
	}
	if err := domain.ValidateConnectorOperation(spec); err != nil {
		return connector.OperationSpec{}, err
	}

	if err := s.operations.SaveConnectorOperation(ctx, spec); err != nil {
		return connector.OperationSpec{}, err
	}

	s.logger.Info("connector operation created", "operation_id", spec.ID.String())
	_ = s.applyConnectorToOperation(ctx, &spec)
	return spec, nil
}

func (s *Service) UpdateConnectorOperation(ctx context.Context, spec connector.OperationSpec) (connector.OperationSpec, error) {
	if spec.TenantID.Empty() {
		return connector.OperationSpec{}, domain.ValidateConnectorOperation(spec)
	}
	if spec.ID.Empty() {
		return connector.OperationSpec{}, domain.ValidateConnectorOperation(spec)
	}

	current, err := s.operations.GetConnectorOperation(ctx, spec.TenantID, spec.ID)
	if err != nil {
		return connector.OperationSpec{}, err
	}
	if spec.Type == "" {
		spec.Type = connector.OperationTypeHTTP
	}
	if spec.Status == "" {
		spec.Status = current.Status
	}
	if spec.Status == "" {
		spec.Status = connector.OperationStatusDraft
	}
	applyConnectorOperationDefaults(&spec, &current)
	if err := s.normalizeConnectorOperationForStorage(ctx, &spec); err != nil {
		return connector.OperationSpec{}, err
	}
	if err := domain.ValidateConnectorOperation(spec); err != nil {
		return connector.OperationSpec{}, err
	}
	if err := s.operations.SaveConnectorOperation(ctx, spec); err != nil {
		return connector.OperationSpec{}, err
	}

	s.logger.Info("connector operation updated", "operation_id", spec.ID.String(), "status", spec.Status)
	_ = s.applyConnectorToOperation(ctx, &spec)
	return spec, nil
}

func (s *Service) SetConnectorOperationStatus(ctx context.Context, tenantID tenant.ID, operationID id.ID, status connector.OperationStatus) (connector.OperationSpec, error) {
	if tenantID.Empty() {
		return connector.OperationSpec{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if operationID.Empty() {
		return connector.OperationSpec{}, apperrors.New(apperrors.CodeInvalidArgument, "operation_id is required")
	}
	if status != connector.OperationStatusEnabled && status != connector.OperationStatusDisabled {
		return connector.OperationSpec{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector operation status transition")
	}

	spec, err := s.operations.GetConnectorOperation(ctx, tenantID, operationID)
	if err != nil {
		return connector.OperationSpec{}, err
	}
	spec.Status = status
	if err := s.operations.SaveConnectorOperation(ctx, spec); err != nil {
		return connector.OperationSpec{}, err
	}
	_ = s.applyConnectorToOperation(ctx, &spec)

	s.logger.Info("connector operation status changed", "operation_id", operationID.String(), "status", status)
	return spec, nil
}

func (s *Service) ListConnectorOperations(ctx context.Context, tenantID tenant.ID) ([]connector.OperationSpec, error) {
	operations, err := s.operations.ListConnectorOperations(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for index := range operations {
		_ = s.applyConnectorToOperation(ctx, &operations[index])
	}
	return operations, nil
}

func (s *Service) GetConnectorOperation(ctx context.Context, tenantID tenant.ID, operationID id.ID) (connector.OperationSpec, error) {
	spec, err := s.operations.GetConnectorOperation(ctx, tenantID, operationID)
	if err != nil {
		return connector.OperationSpec{}, err
	}
	_ = s.applyConnectorToOperation(ctx, &spec)
	return spec, nil
}

func (s *Service) GetConnectorOperationDependencies(ctx context.Context, tenantID tenant.ID, operationID id.ID) (connector.OperationDependencies, error) {
	if _, err := s.operations.GetConnectorOperation(ctx, tenantID, operationID); err != nil {
		return connector.OperationDependencies{}, err
	}

	tools, err := s.tools.ListTools(ctx, tenantID)
	if err != nil {
		return connector.OperationDependencies{}, err
	}
	skills, err := s.skills.ListSkills(ctx, tenantID)
	if err != nil {
		return connector.OperationDependencies{}, err
	}
	agents, err := s.agents.ListAgents(ctx, tenantID)
	if err != nil {
		return connector.OperationDependencies{}, err
	}

	report := connector.OperationDependencies{
		OperationID:    operationID,
		DirectTools:    []connector.ToolConsumer{},
		IndirectSkills: []connector.SkillConsumer{},
		IndirectAgents: []connector.AgentConsumer{},
	}
	toolIDs := map[id.ID]struct{}{}
	for _, spec := range tools {
		if spec.Implementation != tool.ImplementationConnector || spec.Binding.ConnectorOperationID != operationID {
			continue
		}
		toolIDs[spec.ID] = struct{}{}
		requiresReview := spec.RequiresExecutionConfirmation()
		if requiresReview {
			report.Summary.BlockingToolCount++
		}
		report.DirectTools = append(report.DirectTools, connector.ToolConsumer{
			ID:             spec.ID,
			Name:           spec.Name,
			Description:    spec.Description,
			RequiresReview: requiresReview,
		})
	}

	skillIDs := map[id.ID]struct{}{}
	for _, spec := range skills {
		for _, toolID := range spec.ToolIDs {
			if _, ok := toolIDs[toolID]; !ok {
				continue
			}
			skillIDs[spec.ID] = struct{}{}
			report.IndirectSkills = append(report.IndirectSkills, connector.SkillConsumer{
				ID:        spec.ID,
				Name:      spec.Name,
				ViaToolID: toolID,
			})
			break
		}
	}

	for _, profile := range agents {
		for _, skillID := range profile.SkillIDs {
			if _, ok := skillIDs[skillID]; !ok {
				continue
			}
			report.IndirectAgents = append(report.IndirectAgents, connector.AgentConsumer{
				ID:         profile.ID,
				Name:       profile.Name,
				ViaSkillID: skillID,
			})
			break
		}
	}

	report.Summary.DirectToolCount = len(report.DirectTools)
	report.Summary.IndirectSkillCount = len(report.IndirectSkills)
	report.Summary.IndirectAgentCount = len(report.IndirectAgents)
	report.Summary.TotalConsumerCount = report.Summary.DirectToolCount + report.Summary.IndirectSkillCount + report.Summary.IndirectAgentCount
	return report, nil
}

func applyConnectorDefaults(spec *connector.Spec, fallback *connector.Spec) {
	if spec.BusinessDomain == "" && fallback != nil {
		spec.BusinessDomain = fallback.BusinessDomain
	}
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = "General"
	}
	if spec.OwnerTeam == "" && fallback != nil {
		spec.OwnerTeam = fallback.OwnerTeam
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = "Integration Team"
	}
	if spec.TimeoutMillis == 0 && fallback != nil {
		spec.TimeoutMillis = fallback.TimeoutMillis
	}
	if spec.TimeoutMillis == 0 {
		spec.TimeoutMillis = 10_000
	}
	if spec.Headers == nil && fallback != nil {
		spec.Headers = fallback.Headers
	}
	if spec.Headers == nil {
		spec.Headers = map[string]string{}
	}
	if fallback != nil && spec.Auth.Type == "" && spec.Auth.HeaderName == "" && spec.Auth.SecretRef == "" {
		spec.Auth = fallback.Auth
	}
	if spec.Auth.Type == "" {
		spec.Auth.Type = connector.AuthTypeNone
	}
}

func (s *Service) applyConnectorToOperation(ctx context.Context, spec *connector.OperationSpec) error {
	if spec.ConnectorID.Empty() || s.connectors == nil {
		return nil
	}
	parent, err := s.connectors.GetConnector(ctx, spec.TenantID, spec.ConnectorID)
	if err != nil {
		return err
	}
	spec.BaseURL = parent.BaseURL
	spec.Headers = mergeConnectorHeaders(parent.Headers, spec.Headers)
	spec.Auth = parent.Auth
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = parent.BusinessDomain
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = parent.OwnerTeam
	}
	if spec.TimeoutMillis == 0 {
		spec.TimeoutMillis = parent.TimeoutMillis
	}
	return nil
}

// normalizeConnectorOperationForStorage keeps the persisted operation focused on
// the concrete API operation. Connector-owned connection, authentication, and
// ownership fields are inherited dynamically when the operation is returned.
func (s *Service) normalizeConnectorOperationForStorage(ctx context.Context, spec *connector.OperationSpec) error {
	if spec.ConnectorID.Empty() {
		return nil
	}
	if s.connectors == nil {
		return apperrors.New(apperrors.CodeUnavailable, "connector repository is not configured")
	}
	if _, err := s.connectors.GetConnector(ctx, spec.TenantID, spec.ConnectorID); err != nil {
		return err
	}

	spec.BusinessDomain = ""
	spec.OwnerTeam = ""
	spec.BaseURL = ""
	spec.Headers = map[string]string{}
	spec.Auth = connector.AuthConfig{}
	spec.ImplementationMode = connector.ImplementationModeSimpleHTTP
	return nil
}

func mergeConnectorHeaders(parent, operation map[string]string) map[string]string {
	result := map[string]string{}
	for name, value := range parent {
		result[name] = value
	}
	for name, value := range operation {
		result[name] = value
	}
	return result
}

func (s *Service) connectorToolsByOperationID(ctx context.Context, tenantID tenant.ID) (map[id.ID]tool.Spec, error) {
	tools, err := s.tools.ListTools(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	result := map[id.ID]tool.Spec{}
	for _, spec := range tools {
		if spec.Implementation != tool.ImplementationConnector || spec.Binding.ConnectorOperationID.Empty() {
			continue
		}
		if _, exists := result[spec.Binding.ConnectorOperationID]; exists {
			continue
		}
		result[spec.Binding.ConnectorOperationID] = spec
	}
	return result, nil
}

func newToolSpecFromConnectorOperation(connectorSpec connector.Spec, operation connector.OperationSpec) tool.Spec {
	sideEffect := connectorOperationSideEffect(operation)
	return tool.Spec{
		TenantID:       operation.TenantID,
		Name:           operation.Name,
		Description:    operation.Description,
		BusinessDomain: connectorSpec.BusinessDomain,
		OwnerTeam:      connectorSpec.OwnerTeam,
		LLMDescription: operation.Description,
		Implementation: tool.ImplementationConnector,
		Binding: tool.Binding{
			ConnectorOperationID: operation.ID,
		},
		InputSchema:          operation.InputSchema,
		OutputSchema:         operation.OutputSchema,
		SideEffect:           sideEffect,
		RiskLevel:            connectorOperationRiskLevel(sideEffect),
		RequiresConfirmation: sideEffect == tool.SideEffectWrite,
		TimeoutMillis:        operation.TimeoutMillis,
		Status:               tool.StatusDisabled,
		Version:              "v1",
	}
}

func applyConnectorToolDefaults(spec *tool.Spec, connectorSpec connector.Spec, operation connector.OperationSpec) {
	if spec.ID.Empty() {
		spec.ID = id.New("tool")
	}
	if spec.Version == "" {
		spec.Version = "v1"
	}
	if spec.Name == "" {
		spec.Name = operation.Name
	}
	if spec.Description == "" {
		spec.Description = operation.Description
	}
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = connectorSpec.BusinessDomain
	}
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = operation.BusinessDomain
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = connectorSpec.OwnerTeam
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = operation.OwnerTeam
	}
	if spec.LLMDescription == "" {
		spec.LLMDescription = spec.Description
	}
	if spec.InputSchema == nil {
		spec.InputSchema = operation.InputSchema
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = operation.OutputSchema
	}
	if spec.SideEffect == "" {
		spec.SideEffect = connectorOperationSideEffect(operation)
	}
	if spec.RiskLevel == "" {
		spec.RiskLevel = connectorOperationRiskLevel(spec.SideEffect)
	}
	if spec.TimeoutMillis == 0 {
		spec.TimeoutMillis = operation.TimeoutMillis
	}
	if spec.TimeoutMillis == 0 {
		spec.TimeoutMillis = connectorSpec.TimeoutMillis
	}
	if spec.Status == "" {
		spec.Status = tool.StatusDisabled
	}
	applyToolDefaults(spec, nil)
}

func connectorOperationSideEffect(operation connector.OperationSpec) tool.SideEffect {
	if connectorOperationLooksReadOnly(operation) {
		return tool.SideEffectRead
	}
	return tool.SideEffectWrite
}

func connectorOperationLooksReadOnly(operation connector.OperationSpec) bool {
	switch strings.ToUpper(operation.Method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	case http.MethodPost:
		// Some third-party providers expose query-like APIs as POST because the
		// request contract is complex. Treat only obvious query intents as read-only.
		text := strings.ToLower(operation.Name + " " + operation.Path + " " + operation.Description)
		for _, keyword := range []string{"search", "query", "list", "get", "read", "lookup", "find", "fetch", "extract", "crawl", "map"} {
			if strings.Contains(text, keyword) {
				return true
			}
		}
	}
	return false
}

func connectorOperationRiskLevel(sideEffect tool.SideEffect) tool.RiskLevel {
	if sideEffect == tool.SideEffectWrite {
		return tool.RiskMedium
	}
	return tool.RiskLow
}

func applyConnectorOperationDefaults(spec *connector.OperationSpec, fallback *connector.OperationSpec) {
	if spec.ConnectorID.Empty() && fallback != nil {
		spec.ConnectorID = fallback.ConnectorID
	}
	if spec.BusinessDomain == "" && fallback != nil {
		spec.BusinessDomain = fallback.BusinessDomain
	}
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = "General"
	}
	if spec.OwnerTeam == "" && fallback != nil {
		spec.OwnerTeam = fallback.OwnerTeam
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = "Integration Team"
	}
	if spec.ImplementationMode == "" && fallback != nil {
		spec.ImplementationMode = fallback.ImplementationMode
	}
	if spec.ImplementationMode == "" {
		spec.ImplementationMode = connector.ImplementationModeSimpleHTTP
	}
	if spec.TimeoutMillis == 0 && fallback != nil {
		spec.TimeoutMillis = fallback.TimeoutMillis
	}
	if spec.TimeoutMillis == 0 {
		spec.TimeoutMillis = 10_000
	}
	if spec.BaseURL == "" && fallback != nil {
		spec.BaseURL = fallback.BaseURL
	}
	if spec.Method == "" && fallback != nil {
		spec.Method = fallback.Method
	}
	if spec.Path == "" && fallback != nil {
		spec.Path = fallback.Path
	}
	if spec.Headers == nil && fallback != nil {
		spec.Headers = fallback.Headers
	}
	if spec.Headers == nil {
		spec.Headers = map[string]string{}
	}
	if fallback != nil && spec.Auth.Type == "" && spec.Auth.HeaderName == "" && spec.Auth.SecretRef == "" {
		spec.Auth = fallback.Auth
	}
	if spec.Auth.Type == "" {
		spec.Auth.Type = connector.AuthTypeNone
	}
	if spec.InputSchema == nil && fallback != nil {
		spec.InputSchema = fallback.InputSchema
	}
	if spec.InputSchema == nil {
		spec.InputSchema = map[string]any{}
	}
	if spec.OutputSchema == nil && fallback != nil {
		spec.OutputSchema = fallback.OutputSchema
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = map[string]any{}
	}
}

func applyToolDefaults(spec *tool.Spec, fallback *tool.Spec) {
	if spec.Status == "" && fallback != nil {
		spec.Status = fallback.Status
	}
	if spec.Status == "" {
		spec.Status = tool.StatusDisabled
	}
	if spec.BusinessDomain == "" && fallback != nil {
		spec.BusinessDomain = fallback.BusinessDomain
	}
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = "General"
	}
	if spec.OwnerTeam == "" && fallback != nil {
		spec.OwnerTeam = fallback.OwnerTeam
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = "AI Platform"
	}
	if spec.LLMDescription == "" && fallback != nil {
		spec.LLMDescription = fallback.LLMDescription
	}
	if spec.LLMDescription == "" {
		spec.LLMDescription = spec.Description
	}
	if spec.SideEffect == "" && fallback != nil {
		spec.SideEffect = fallback.SideEffect
	}
	if spec.SideEffect == "" {
		spec.SideEffect = tool.SideEffectNone
	}
	if spec.RiskLevel == "" && fallback != nil {
		spec.RiskLevel = fallback.RiskLevel
	}
	if spec.RiskLevel == "" {
		spec.RiskLevel = tool.RiskLow
	}
	if spec.TimeoutMillis == 0 && fallback != nil {
		spec.TimeoutMillis = fallback.TimeoutMillis
	}
	if spec.TimeoutMillis == 0 {
		spec.TimeoutMillis = 10_000
	}
	if spec.RetryPolicy.MaxAttempts == 0 && spec.RetryPolicy.BackoffMillis == 0 && fallback != nil {
		spec.RetryPolicy = fallback.RetryPolicy
	}
}

func applySkillDefaults(spec *skill.Spec, fallback *skill.Spec) {
	if spec.Status == "" && fallback != nil {
		spec.Status = fallback.Status
	}
	if spec.Status == "" {
		spec.Status = skill.StatusDraft
	}
	if spec.BusinessDomain == "" && fallback != nil {
		spec.BusinessDomain = fallback.BusinessDomain
	}
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = "General"
	}
	if spec.OwnerTeam == "" && fallback != nil {
		spec.OwnerTeam = fallback.OwnerTeam
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = "AI Platform"
	}
	if spec.RiskLevel == "" && fallback != nil {
		spec.RiskLevel = fallback.RiskLevel
	}
	if spec.RiskLevel == "" {
		spec.RiskLevel = skill.RiskLow
	}
	if spec.ExecutionPolicy.MaxToolCalls == 0 && fallback != nil {
		spec.ExecutionPolicy.MaxToolCalls = fallback.ExecutionPolicy.MaxToolCalls
	}
	if spec.ExecutionPolicy.MaxToolCalls == 0 {
		spec.ExecutionPolicy.MaxToolCalls = 4
	}
	if spec.ExecutionPolicy.TimeoutMillis == 0 && fallback != nil {
		spec.ExecutionPolicy.TimeoutMillis = fallback.ExecutionPolicy.TimeoutMillis
	}
	if spec.ExecutionPolicy.TimeoutMillis == 0 {
		spec.ExecutionPolicy.TimeoutMillis = 30_000
	}
}

func applyAgentDefaults(profile *agent.Profile, fallback *agent.Profile) {
	if profile.Status == "" && fallback != nil {
		profile.Status = fallback.Status
	}
	if profile.Status == "" {
		profile.Status = agent.StatusDraft
	}
	if profile.BusinessDomain == "" && fallback != nil {
		profile.BusinessDomain = fallback.BusinessDomain
	}
	if profile.BusinessDomain == "" {
		profile.BusinessDomain = "General"
	}
	if profile.OwnerTeam == "" && fallback != nil {
		profile.OwnerTeam = fallback.OwnerTeam
	}
	if profile.OwnerTeam == "" {
		profile.OwnerTeam = "AI Platform"
	}
	if profile.DefaultLang == "" && fallback != nil {
		profile.DefaultLang = fallback.DefaultLang
	}
	if profile.DefaultLang == "" {
		profile.DefaultLang = "zh-CN"
	}
	if len(profile.SupportedLanguages) == 0 && fallback != nil {
		profile.SupportedLanguages = fallback.SupportedLanguages
	}
	if len(profile.SupportedLanguages) == 0 {
		profile.SupportedLanguages = []string{profile.DefaultLang}
	}
	if len(profile.Channels) == 0 && fallback != nil {
		profile.Channels = fallback.Channels
	}
	if len(profile.Channels) == 0 {
		profile.Channels = []string{"text", "voice"}
	}
	if profile.ModelConfig.ProviderID.Empty() && profile.ModelConfig.Model == "" && fallback != nil {
		profile.ModelConfig = fallback.ModelConfig
	}
	if profile.ModelConfig.Model == "" {
		profile.ModelConfig.Model = "default"
	}
	if profile.RuntimePolicy.MaxTurns == 0 && fallback != nil {
		profile.RuntimePolicy.MaxTurns = fallback.RuntimePolicy.MaxTurns
	}
	if profile.RuntimePolicy.MaxTurns == 0 {
		profile.RuntimePolicy.MaxTurns = 12
	}
	if profile.RuntimePolicy.MaxToolCalls == 0 && fallback != nil {
		profile.RuntimePolicy.MaxToolCalls = fallback.RuntimePolicy.MaxToolCalls
	}
	if profile.RuntimePolicy.MaxToolCalls == 0 {
		profile.RuntimePolicy.MaxToolCalls = 6
	}
	if profile.RuntimePolicy.ResponseTimeoutMs == 0 && fallback != nil {
		profile.RuntimePolicy.ResponseTimeoutMs = fallback.RuntimePolicy.ResponseTimeoutMs
	}
	if profile.RuntimePolicy.ResponseTimeoutMs == 0 {
		profile.RuntimePolicy.ResponseTimeoutMs = 30_000
	}
}

func applyWorkflowDefaults(spec *workflow.Spec, fallback *workflow.Spec) {
	if spec.Status == "" && fallback != nil {
		spec.Status = fallback.Status
	}
	if spec.Status == "" {
		spec.Status = workflow.StatusDraft
	}
	if spec.Profile == "" && fallback != nil {
		spec.Profile = fallback.Profile
	}
	if spec.Profile == "" {
		spec.Profile = workflow.ProfileToolWorkflow
	}
	if spec.BusinessDomain == "" && fallback != nil {
		spec.BusinessDomain = fallback.BusinessDomain
	}
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = "General"
	}
	if spec.OwnerTeam == "" && fallback != nil {
		spec.OwnerTeam = fallback.OwnerTeam
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = "AI Platform"
	}
	if workflowGraphIsEmpty(spec.Graph) && fallback != nil {
		spec.Graph = fallback.Graph
	}
	if spec.ContextSchema == nil && fallback != nil {
		spec.ContextSchema = fallback.ContextSchema
	}
	if spec.ContextSchema == nil {
		spec.ContextSchema = map[string]any{}
	}
	if spec.InputSchema == nil && fallback != nil {
		spec.InputSchema = fallback.InputSchema
	}
	if spec.InputSchema == nil {
		spec.InputSchema = map[string]any{}
	}
	if spec.OutputSchema == nil && fallback != nil {
		spec.OutputSchema = fallback.OutputSchema
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = map[string]any{}
	}
	if spec.UI == nil && fallback != nil {
		spec.UI = fallback.UI
	}
	if spec.UI == nil {
		spec.UI = map[string]any{}
	}
	if spec.Policy.MaxSteps == 0 {
		spec.Policy.MaxSteps = 64
	}
	if spec.Policy.MaxParallelism == 0 {
		spec.Policy.MaxParallelism = 4
	}
	normalizeWorkflowGraph(spec)
}

func applyAgentFlowDefaults(spec *agentflow.Spec, fallback *agentflow.Spec) {
	if spec.Status == "" && fallback != nil {
		spec.Status = fallback.Status
	}
	if spec.Status == "" {
		spec.Status = agentflow.StatusDraft
	}
	if spec.OrchestrationMode == "" && fallback != nil {
		spec.OrchestrationMode = fallback.OrchestrationMode
	}
	if spec.OrchestrationMode == "" {
		spec.OrchestrationMode = agentflow.OrchestrationModeWorkflow
	}
	if supervisorSpecIsEmpty(spec.Supervisor) && fallback != nil {
		spec.Supervisor = fallback.Supervisor
	}
	applySupervisorDefaults(&spec.Supervisor)
	if spec.BusinessDomain == "" && fallback != nil {
		spec.BusinessDomain = fallback.BusinessDomain
	}
	if spec.BusinessDomain == "" {
		spec.BusinessDomain = "General"
	}
	if spec.OwnerTeam == "" && fallback != nil {
		spec.OwnerTeam = fallback.OwnerTeam
	}
	if spec.OwnerTeam == "" {
		spec.OwnerTeam = "AI Platform"
	}
	if flowGraphIsEmpty(spec.Graph) && fallback != nil {
		spec.Graph = fallback.Graph
	}
	if spec.ContextSchema == nil && fallback != nil {
		spec.ContextSchema = fallback.ContextSchema
	}
	if spec.ContextSchema == nil {
		spec.ContextSchema = map[string]any{}
	}
	spec.InputSchema = fixedAgentFlowInputSchema()
	spec.OutputSchema = fixedAgentFlowOutputSchema()
	normalizeAgentFlowGraph(spec)
	normalizeSupervisorFromGraph(spec)
}

func applySupervisorDefaults(spec *agentflow.SupervisorSpec) {
	if spec.MaxDepth == 0 {
		spec.MaxDepth = 4
	}
	if spec.MaxSubAgentCalls == 0 {
		spec.MaxSubAgentCalls = 5
	}
}

func supervisorSpecIsEmpty(spec agentflow.SupervisorSpec) bool {
	return spec.SupervisorAgentID.Empty() &&
		len(spec.SubAgentIDs) == 0 &&
		spec.MaxDepth == 0 &&
		spec.MaxSubAgentCalls == 0 &&
		strings.TrimSpace(spec.PlanningPrompt) == "" &&
		strings.TrimSpace(spec.FinalPrompt) == ""
}

func (s *Service) materializeLocalAgentFlowNodes(ctx context.Context, spec *agentflow.Spec) error {
	if spec == nil || len(spec.Graph.Nodes) == 0 {
		return nil
	}
	for nodeID, node := range spec.Graph.Nodes {
		profile, ok := localAgentProfileFromNode(*spec, node)
		if !ok {
			continue
		}
		if s.agents == nil {
			return apperrors.New(apperrors.CodeUnavailable, "agent repository is required for local agent nodes")
		}
		applyAgentDefaults(&profile, nil)
		if err := domain.ValidateAgent(profile); err != nil {
			return err
		}
		if err := s.agents.SaveAgent(ctx, profile); err != nil {
			return err
		}
		if node.Config == nil {
			node.Config = map[string]any{}
		}
		node.Config["agent_id"] = profile.ID.String()
		node.Config["agent_mode"] = "local"
		spec.Graph.Nodes[nodeID] = node
	}
	return nil
}

func localAgentProfileFromNode(spec agentflow.Spec, node flowdomain.Node) (agent.Profile, bool) {
	localConfig, ok := localAgentConfigFromNode(node)
	if !ok {
		return agent.Profile{}, false
	}
	name := stringFromAny(localConfig["name"])
	if strings.TrimSpace(name) == "" {
		name = node.Name
	}
	description := stringFromAny(localConfig["description"])
	if strings.TrimSpace(description) == "" {
		description = node.Description
	}
	return agent.Profile{
		ID:             localAgentID(spec.ID, node.ID),
		TenantID:       spec.TenantID,
		Name:           name,
		Description:    description,
		BusinessDomain: spec.BusinessDomain,
		OwnerTeam:      flowLocalAgentOwnerTeam,
		Status:         agent.StatusEnabled,
		SkillIDs:       idSliceFromAny(localConfig["skillIds"]),
		ToolIDs:        idSliceFromAny(localConfig["toolIds"]),
		DefaultLang:    "zh-CN",
		SystemPrompt:   stringFromAny(localConfig["systemPrompt"]),
		ModelConfig: agent.ModelConfig{
			Model:       stringFromAny(localConfig["model"]),
			Temperature: numberFromAny(localConfig["temperature"]),
		},
		RuntimePolicy: agent.RuntimePolicy{
			MaxTurns:          12,
			MaxToolCalls:      6,
			ResponseTimeoutMs: 30000,
		},
		Version: spec.Version,
	}, true
}

func localAgentConfigFromNode(node flowdomain.Node) (map[string]any, bool) {
	if node.Type != flowdomain.NodeTypeAgent && node.Type != flowdomain.NodeTypeSupervisor {
		return nil, false
	}
	if configString(node.Config, "agent_mode") != "local" {
		return nil, false
	}
	value, ok := node.Config["local_agent"]
	if !ok {
		return nil, false
	}
	localConfig, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return localConfig, true
}

func localAgentID(flowID id.ID, nodeID id.ID) id.ID {
	return id.ID("agent_local_" + flowID.String() + "_" + nodeID.String())
}

func isFlowLocalAgent(profile agent.Profile) bool {
	return profile.OwnerTeam == flowLocalAgentOwnerTeam || strings.HasPrefix(profile.ID.String(), "agent_local_flow_")
}

func configString(config map[string]any, key string) string {
	value, ok := config[key]
	if !ok {
		return ""
	}
	return stringFromAny(value)
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func idSliceFromAny(value any) []id.ID {
	values, ok := value.([]any)
	if !ok {
		if stringValues, ok := value.([]string); ok {
			result := make([]id.ID, 0, len(stringValues))
			for _, item := range stringValues {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					result = append(result, id.ID(trimmed))
				}
			}
			return result
		}
		return []id.ID{}
	}
	result := make([]id.ID, 0, len(values))
	for _, value := range values {
		if text := stringFromAny(value); text != "" {
			result = append(result, id.ID(text))
		}
	}
	return result
}

func numberFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func normalizeSupervisorFromGraph(spec *agentflow.Spec) {
	applySupervisorDefaults(&spec.Supervisor)
	if spec.OrchestrationMode != agentflow.OrchestrationModeSupervisor {
		return
	}
	supervisorAgentID, subAgentIDs, ok := deriveSupervisorAgentsFromGraph(spec.Graph)
	if !ok {
		return
	}
	spec.Supervisor.SupervisorAgentID = supervisorAgentID
	spec.Supervisor.SubAgentIDs = subAgentIDs
	if graphDepth := agentGraphDepth(spec.Graph); graphDepth > spec.Supervisor.MaxDepth {
		spec.Supervisor.MaxDepth = graphDepth
	}
}

func deriveSupervisorAgentsFromGraph(graph flowdomain.FlowGraph) (id.ID, []id.ID, bool) {
	supervisorNodeID, ok := supervisorNodeIDFromGraph(graph)
	if !ok {
		if len(graph.Nodes) > 0 {
			return "", nil, true
		}
		return "", nil, false
	}
	supervisorNode, ok := graph.Nodes[supervisorNodeID]
	if !ok {
		return "", nil, true
	}

	supervisorAgentID := agentIDFromFlowNode(supervisorNode)
	nodeIDs := make([]string, 0, len(graph.Nodes))
	for nodeID := range graph.Nodes {
		nodeIDs = append(nodeIDs, nodeID.String())
	}
	sort.Strings(nodeIDs)

	seen := map[string]struct{}{}
	subAgentIDs := make([]id.ID, 0)
	for _, nodeID := range nodeIDs {
		node := graph.Nodes[id.ID(nodeID)]
		if node.ID == supervisorNodeID || !isAgentLikeFlowNode(node) {
			continue
		}
		agentID := agentIDFromFlowNode(node)
		if agentID.Empty() || agentID == supervisorAgentID {
			continue
		}
		if _, exists := seen[agentID.String()]; exists {
			continue
		}
		seen[agentID.String()] = struct{}{}
		subAgentIDs = append(subAgentIDs, agentID)
	}
	return supervisorAgentID, subAgentIDs, true
}

func supervisorNodeIDFromGraph(graph flowdomain.FlowGraph) (id.ID, bool) {
	entryNodeID := graph.EntryNodeID
	if entryNodeID.Empty() {
		entryNodeID = id.ID("start")
	}
	for _, edge := range graph.Edges {
		if edge.FromNodeID == entryNodeID {
			return edge.ToNodeID, true
		}
	}
	return "", false
}

func isAgentLikeFlowNode(node flowdomain.Node) bool {
	return node.Type == flowdomain.NodeTypeAgent || node.Type == flowdomain.NodeTypeSupervisor
}

func agentIDFromFlowNode(node flowdomain.Node) id.ID {
	value, ok := node.Config["agent_id"]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return id.ID(strings.TrimSpace(text))
}

func agentGraphDepth(graph flowdomain.FlowGraph) int {
	entryNodeID := graph.EntryNodeID
	if entryNodeID.Empty() {
		entryNodeID = id.ID("start")
	}
	supervisorNodeID, ok := supervisorNodeIDFromGraph(graph)
	if !ok {
		return 0
	}
	childrenByNodeID := map[id.ID][]id.ID{}
	for _, edge := range graph.Edges {
		if edge.FromNodeID == entryNodeID {
			continue
		}
		childrenByNodeID[edge.FromNodeID] = append(childrenByNodeID[edge.FromNodeID], edge.ToNodeID)
	}
	visiting := map[id.ID]bool{}
	var depth func(nodeID id.ID) int
	depth = func(nodeID id.ID) int {
		if visiting[nodeID] {
			return 17
		}
		visiting[nodeID] = true
		defer delete(visiting, nodeID)
		maxChildDepth := 0
		for _, childID := range childrenByNodeID[nodeID] {
			childDepth := 1 + depth(childID)
			if childDepth > maxChildDepth {
				maxChildDepth = childDepth
			}
		}
		return maxChildDepth
	}
	return depth(supervisorNodeID)
}

func normalizeAgentFlowGraph(spec *agentflow.Spec) {
	spec.Graph.ID = spec.ID
	spec.Graph.TenantID = spec.TenantID
	spec.Graph.Name = spec.Name
	spec.Graph.Description = spec.Description
	spec.Graph.Status = spec.Status
	spec.Graph.Version = spec.Version
	if spec.Graph.Nodes == nil {
		spec.Graph.Nodes = map[id.ID]flowdomain.Node{}
	}
	if spec.Graph.EntryNodeID.Empty() {
		spec.Graph.EntryNodeID = id.ID("start")
	}
	if _, ok := spec.Graph.Nodes[spec.Graph.EntryNodeID]; !ok {
		spec.Graph.Nodes[spec.Graph.EntryNodeID] = flowdomain.Node{
			ID:   spec.Graph.EntryNodeID,
			Type: flowdomain.NodeTypeStart,
			Name: "Start",
		}
	}
}

func normalizeWorkflowGraph(spec *workflow.Spec) {
	if spec.Graph.Nodes == nil {
		spec.Graph.Nodes = map[id.ID]workflow.Node{}
	}
	if spec.Graph.EntryNodeID.Empty() {
		spec.Graph.EntryNodeID = id.ID("start")
	}
	if _, ok := spec.Graph.Nodes[spec.Graph.EntryNodeID]; !ok {
		spec.Graph.Nodes[spec.Graph.EntryNodeID] = workflow.Node{
			ID:   spec.Graph.EntryNodeID,
			Type: workflow.NodeTypeStart,
			Name: "Start",
		}
	}
}

func flowGraphIsEmpty(graph flowdomain.FlowGraph) bool {
	return graph.ID.Empty() &&
		graph.TenantID.Empty() &&
		strings.TrimSpace(graph.Name) == "" &&
		graph.EntryNodeID.Empty() &&
		len(graph.Nodes) == 0 &&
		len(graph.Edges) == 0
}

func workflowGraphIsEmpty(graph workflow.Graph) bool {
	return graph.EntryNodeID.Empty() &&
		len(graph.Nodes) == 0 &&
		len(graph.Edges) == 0
}
