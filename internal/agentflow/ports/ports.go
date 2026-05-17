package ports

import (
	"context"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type NodeExecutionRequest struct {
	Run     domain.FlowRun
	Node    domain.Node
	Context domain.RunContext
}

type NodeExecutor interface {
	ExecuteNode(ctx context.Context, request NodeExecutionRequest) (domain.NodeResult, error)
}

type AgentInvocationRequest struct {
	Run     domain.FlowRun
	Node    domain.Node
	Context domain.RunContext
	AgentID id.ID
	// RuntimeSystemPrompt carries trusted orchestration instructions that should
	// be appended to the Agent system prompt instead of mixed into user input.
	RuntimeSystemPrompt string
	Task                string
	Payload             map[string]any
	TraceID             string
	SessionID           id.ID
	UserID              string
}

type AgentInvocationResult struct {
	Text     string
	Response event.Response
	Actions  []event.Action
	TraceID  string
	Raw      map[string]any
}

type AgentInvoker interface {
	InvokeAgent(ctx context.Context, request AgentInvocationRequest) (AgentInvocationResult, error)
}

type AgentCatalog interface {
	GetAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Profile, error)
}

type AgentCapabilityConfig struct {
	Agent  agent.Profile
	Skills []skill.Spec
	Tools  []tool.Spec
}

type AgentCapabilityCatalog interface {
	LoadAgentCapabilityConfig(ctx context.Context, tenantID tenant.ID, agentID id.ID) (AgentCapabilityConfig, error)
}

type AgentFlowConfigLoader interface {
	LoadAgentFlow(ctx context.Context, tenantID tenant.ID, flowID id.ID) (agentflow.Spec, error)
}

type RunStore interface {
	CreateRun(ctx context.Context, run domain.FlowRun) error
	UpdateRun(ctx context.Context, run domain.FlowRun) error
	GetRun(ctx context.Context, tenantID tenant.ID, runID id.ID) (domain.FlowRun, error)
	RecordNodeRun(ctx context.Context, nodeRun domain.NodeRun) error
	ListNodeRuns(ctx context.Context, tenantID tenant.ID, runID id.ID) ([]domain.NodeRun, error)
}

type TraceEmitter interface {
	FlowRunStarted(ctx context.Context, run domain.FlowRun)
	FlowRunFinished(ctx context.Context, run domain.FlowRun)
	NodeRunStarted(ctx context.Context, nodeRun domain.NodeRun)
	NodeRunFinished(ctx context.Context, nodeRun domain.NodeRun)
}
