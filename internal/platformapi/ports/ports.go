package ports

import (
	"context"

	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type AgentRepository interface {
	SaveAgent(ctx context.Context, profile agent.Profile) error
	GetAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Profile, error)
	ListAgents(ctx context.Context, tenantID tenant.ID) ([]agent.Profile, error)
}

type AgentFlowRepository interface {
	SaveAgentFlow(ctx context.Context, spec agentflow.Spec) error
	GetAgentFlow(ctx context.Context, tenantID tenant.ID, flowID id.ID) (agentflow.Spec, error)
	ListAgentFlows(ctx context.Context, tenantID tenant.ID) ([]agentflow.Spec, error)
}

type ToolRepository interface {
	SaveTool(ctx context.Context, spec tool.Spec) error
	GetTool(ctx context.Context, tenantID tenant.ID, toolID id.ID) (tool.Spec, error)
	ListTools(ctx context.Context, tenantID tenant.ID) ([]tool.Spec, error)
}

type SkillRepository interface {
	SaveSkill(ctx context.Context, spec skill.Spec) error
	GetSkill(ctx context.Context, tenantID tenant.ID, skillID id.ID) (skill.Spec, error)
	ListSkills(ctx context.Context, tenantID tenant.ID) ([]skill.Spec, error)
}

type ConnectorRepository interface {
	SaveConnector(ctx context.Context, spec connector.Spec) error
	GetConnector(ctx context.Context, tenantID tenant.ID, connectorID id.ID) (connector.Spec, error)
	ListConnectors(ctx context.Context, tenantID tenant.ID) ([]connector.Spec, error)
}

type ConnectorOperationRepository interface {
	SaveConnectorOperation(ctx context.Context, spec connector.OperationSpec) error
	GetConnectorOperation(ctx context.Context, tenantID tenant.ID, operationID id.ID) (connector.OperationSpec, error)
	ListConnectorOperations(ctx context.Context, tenantID tenant.ID) ([]connector.OperationSpec, error)
}

type WorkflowRepository interface {
	SaveWorkflow(ctx context.Context, spec workflow.Spec) error
	GetWorkflow(ctx context.Context, tenantID tenant.ID, workflowID id.ID) (workflow.Spec, error)
	ListWorkflows(ctx context.Context, tenantID tenant.ID) ([]workflow.Spec, error)
}
