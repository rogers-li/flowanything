package ports

import (
	"context"

	"flow-anything/internal/aiorchestrator/domain"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/contracts/runtimeevent"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type AgentConfigReader interface {
	GetAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Profile, error)
}

type AgentConfigLoader interface {
	LoadAgentConfig(ctx context.Context, tenantID tenant.ID, agentID id.ID) (domain.AgentConfig, error)
}

type SkillReader interface {
	GetSkill(ctx context.Context, tenantID tenant.ID, skillID id.ID) (skill.Spec, error)
}

type ToolCatalogReader interface {
	ListTools(ctx context.Context, tenantID tenant.ID, agentID id.ID) ([]tool.Spec, error)
}

type ToolReader interface {
	GetTool(ctx context.Context, tenantID tenant.ID, toolID id.ID) (tool.Spec, error)
}

type ToolRuntime interface {
	ExecuteTool(ctx context.Context, call tool.Call) (tool.Result, error)
}

type ModelClient interface {
	Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error)
}

type ConversationStore interface {
	LoadMessages(ctx context.Context, ref domain.ConversationRef, limit int) ([]model.Message, error)
	AppendMessages(ctx context.Context, ref domain.ConversationRef, messages []model.Message) error
}

type TraceStore interface {
	StartTrace(ctx context.Context, trace domain.TraceRecord) error
	AppendStep(ctx context.Context, tenantID tenant.ID, traceID string, step domain.TraceStep) error
	FinishTrace(ctx context.Context, tenantID tenant.ID, traceID string, status domain.TraceStatus, errText string) error
	GetTrace(ctx context.Context, tenantID tenant.ID, traceID string) (domain.TraceRecord, error)
}

type RuntimeEventSink interface {
	Publish(ctx context.Context, event runtimeevent.Event) error
}

type RuntimeEventSubscriber interface {
	Subscribe(ctx context.Context, tenantID tenant.ID, traceID string) (<-chan runtimeevent.Event, func())
}
