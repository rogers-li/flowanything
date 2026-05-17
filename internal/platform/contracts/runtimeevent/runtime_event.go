package runtimeevent

import (
	"time"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Type string

const (
	TypeRunStarted                Type = "run_started"
	TypeRunCompleted              Type = "run_completed"
	TypeRunFailed                 Type = "run_failed"
	TypePlanningStarted           Type = "planning_started"
	TypePlanningCompleted         Type = "planning_completed"
	TypeActionPlanned             Type = "action_planned"
	TypeActionStarted             Type = "action_started"
	TypeActionCompleted           Type = "action_completed"
	TypeActionFailed              Type = "action_failed"
	TypeLLMStarted                Type = "llm_started"
	TypeLLMCompleted              Type = "llm_completed"
	TypeLLMFailed                 Type = "llm_failed"
	TypeTraceStepAdded            Type = "trace_step_added"
	TypeContextAssembled          Type = "context_assembled"
	TypeAssistantMessageCompleted Type = "assistant_message_completed"
)

type Event struct {
	ID        string         `json:"id"`
	Type      Type           `json:"type"`
	TenantID  tenant.ID      `json:"tenant_id"`
	RunID     string         `json:"run_id,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
	EventID   id.ID          `json:"event_id,omitempty"`
	AgentID   id.ID          `json:"agent_id,omitempty"`
	SessionID id.ID          `json:"session_id,omitempty"`
	ParentID  string         `json:"parent_id,omitempty"`
	StepID    string         `json:"step_id,omitempty"`
	StepType  string         `json:"step_type,omitempty"`
	Name      string         `json:"name,omitempty"`
	Status    string         `json:"status,omitempty"`
	Message   string         `json:"message,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}
