package agentflow

import (
	flowdomain "flow-anything/internal/agentflow/domain"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Status = flowdomain.FlowStatus

const (
	StatusDraft    = flowdomain.FlowStatusDraft
	StatusEnabled  = flowdomain.FlowStatusEnabled
	StatusDisabled = flowdomain.FlowStatusDisabled
)

type OrchestrationMode string

const (
	OrchestrationModeWorkflow   OrchestrationMode = "workflow"
	OrchestrationModeSupervisor OrchestrationMode = "supervisor"
)

type SupervisorSpec struct {
	SupervisorAgentID id.ID   `json:"supervisor_agent_id,omitempty"`
	SubAgentIDs       []id.ID `json:"sub_agent_ids,omitempty"`
	MaxDepth          int     `json:"max_depth,omitempty"`
	MaxSubAgentCalls  int     `json:"max_sub_agent_calls,omitempty"`
	PlanningPrompt    string  `json:"planning_prompt,omitempty"`
	FinalPrompt       string  `json:"final_prompt,omitempty"`
}

type Spec struct {
	ID                id.ID                `json:"id"`
	TenantID          tenant.ID            `json:"tenant_id"`
	Name              string               `json:"name"`
	Description       string               `json:"description,omitempty"`
	BusinessDomain    string               `json:"business_domain,omitempty"`
	OwnerTeam         string               `json:"owner_team,omitempty"`
	Status            Status               `json:"status"`
	OrchestrationMode OrchestrationMode    `json:"orchestration_mode,omitempty"`
	Supervisor        SupervisorSpec       `json:"supervisor,omitempty"`
	Graph             flowdomain.FlowGraph `json:"graph"`
	ContextSchema     map[string]any       `json:"context_schema,omitempty"`
	InputSchema       map[string]any       `json:"input_schema,omitempty"`
	OutputSchema      map[string]any       `json:"output_schema,omitempty"`
	Version           string               `json:"version"`
}

func (s Spec) RuntimeGraph() flowdomain.FlowGraph {
	return s.Graph
}

func (s Spec) RuntimeEnabled() bool {
	return s.Status == StatusEnabled
}
