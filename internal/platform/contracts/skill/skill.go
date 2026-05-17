package skill

import (
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusEnabled  Status = "enabled"
	StatusDisabled Status = "disabled"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type ExecutionPolicy struct {
	MaxToolCalls        int  `json:"max_tool_calls"`
	TimeoutMillis       int  `json:"timeout_ms"`
	AllowWriteTools     bool `json:"allow_write_tools"`
	RequireConfirmation bool `json:"require_confirmation"`
}

type Spec struct {
	ID              id.ID           `json:"id"`
	TenantID        tenant.ID       `json:"tenant_id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	BusinessDomain  string          `json:"business_domain,omitempty"`
	OwnerTeam       string          `json:"owner_team,omitempty"`
	Status          Status          `json:"status"`
	ToolIDs         []id.ID         `json:"tool_ids,omitempty"`
	KnowledgeIDs    []id.ID         `json:"knowledge_ids,omitempty"`
	SystemPrompt    string          `json:"system_prompt,omitempty"`
	UseCases        []string        `json:"use_cases,omitempty"`
	Exclusions      []string        `json:"exclusions,omitempty"`
	OutputFormat    string          `json:"output_format,omitempty"`
	RiskLevel       RiskLevel       `json:"risk_level"`
	ExecutionPolicy ExecutionPolicy `json:"execution_policy,omitempty"`
	PolicyVersion   string          `json:"policy_version,omitempty"`
	Version         string          `json:"version"`
}

func (s Spec) RuntimeEnabled() bool {
	return s.Status == "" || s.Status == StatusEnabled
}

type DependencySummary struct {
	DirectAgentCount   int `json:"direct_agent_count"`
	TotalConsumerCount int `json:"total_consumer_count"`
}

type AgentConsumer struct {
	ID   id.ID  `json:"id"`
	Name string `json:"name"`
}

type Dependencies struct {
	SkillID      id.ID             `json:"skill_id"`
	Summary      DependencySummary `json:"summary"`
	DirectAgents []AgentConsumer   `json:"direct_agents"`
}
