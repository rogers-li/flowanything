package agent

import (
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusEnabled  Status = "enabled"
	StatusDisabled Status = "disabled"
)

type ModelConfig struct {
	ProviderID  id.ID   `json:"provider_id,omitempty"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

type RuntimePolicy struct {
	MaxTurns          int `json:"max_turns"`
	MaxToolCalls      int `json:"max_tool_calls"`
	ResponseTimeoutMs int `json:"response_timeout_ms"`
}

type Profile struct {
	ID                 id.ID         `json:"id"`
	TenantID           tenant.ID     `json:"tenant_id"`
	Name               string        `json:"name"`
	Description        string        `json:"description"`
	BusinessDomain     string        `json:"business_domain,omitempty"`
	OwnerTeam          string        `json:"owner_team,omitempty"`
	Status             Status        `json:"status"`
	SkillIDs           []id.ID       `json:"skill_ids,omitempty"`
	ToolIDs            []id.ID       `json:"tool_ids,omitempty"`
	DefaultLang        string        `json:"default_lang,omitempty"`
	SupportedLanguages []string      `json:"supported_languages,omitempty"`
	Channels           []string      `json:"channels,omitempty"`
	SystemPrompt       string        `json:"system_prompt,omitempty"`
	WelcomeMessage     string        `json:"welcome_message,omitempty"`
	ModelConfig        ModelConfig   `json:"model_config,omitempty"`
	RuntimePolicy      RuntimePolicy `json:"runtime_policy,omitempty"`
	Version            string        `json:"version"`
}

func (p Profile) RuntimeEnabled() bool {
	return p.Status == "" || p.Status == StatusEnabled
}

type DependencySummary struct {
	DirectSkillCount     int `json:"direct_skill_count"`
	DirectToolCount      int `json:"direct_tool_count"`
	ReachableToolCount   int `json:"reachable_tool_count"`
	DisabledSkillCount   int `json:"disabled_skill_count"`
	TotalCapabilityCount int `json:"total_capability_count"`
}

type SkillBinding struct {
	ID     id.ID  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

type ToolBinding struct {
	ID             id.ID                   `json:"id"`
	Name           string                  `json:"name"`
	ViaSkillID     id.ID                   `json:"via_skill_id,omitempty"`
	Source         string                  `json:"source,omitempty"`
	Implementation tool.ImplementationType `json:"implementation,omitempty"`
	RiskLevel      tool.RiskLevel          `json:"risk_level,omitempty"`
	Status         string                  `json:"status,omitempty"`
}

type Dependencies struct {
	AgentID        id.ID             `json:"agent_id"`
	Summary        DependencySummary `json:"summary"`
	DirectSkills   []SkillBinding    `json:"direct_skills"`
	ReachableTools []ToolBinding     `json:"reachable_tools"`
}
