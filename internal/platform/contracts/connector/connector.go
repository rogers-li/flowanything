package connector

import (
	"time"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type OperationType string

const (
	OperationTypeHTTP OperationType = "http"
)

type OperationStatus string

const (
	OperationStatusDraft    OperationStatus = "draft"
	OperationStatusEnabled  OperationStatus = "enabled"
	OperationStatusDisabled OperationStatus = "disabled"
)

type Spec struct {
	ID             id.ID             `json:"id"`
	TenantID       tenant.ID         `json:"tenant_id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	BusinessDomain string            `json:"business_domain,omitempty"`
	OwnerTeam      string            `json:"owner_team,omitempty"`
	Type           OperationType     `json:"type"`
	Status         OperationStatus   `json:"status"`
	BaseURL        string            `json:"base_url"`
	Headers        map[string]string `json:"headers,omitempty"`
	Auth           AuthConfig        `json:"auth,omitempty"`
	TimeoutMillis  int               `json:"timeout_ms"`
	Version        string            `json:"version,omitempty"`
}

type ImplementationMode string

const (
	ImplementationModeSimpleHTTP      ImplementationMode = "simple_http"
	ImplementationModeTemplateMapping ImplementationMode = "template_mapping"
	ImplementationModeAdapterService  ImplementationMode = "adapter_service"
	ImplementationModeWorkflow        ImplementationMode = "workflow"
	ImplementationModeMock            ImplementationMode = "mock"
)

type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeBearer AuthType = "bearer"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeOAuth2 AuthType = "oauth2"
)

type AuthConfig struct {
	Type                 AuthType `json:"type"`
	HeaderName           string   `json:"header_name,omitempty"`
	SecretRef            string   `json:"secret_ref,omitempty"`
	Provider             string   `json:"provider,omitempty"`
	ClientIDRef          string   `json:"client_id_ref,omitempty"`
	ClientSecretRef      string   `json:"client_secret_ref,omitempty"`
	RefreshTokenRef      string   `json:"refresh_token_ref,omitempty"`
	AuthorizationCodeRef string   `json:"authorization_code_ref,omitempty"`
	AppAccessTokenURL    string   `json:"app_access_token_url,omitempty"`
	AccessTokenURL       string   `json:"access_token_url,omitempty"`
	RefreshTokenURL      string   `json:"refresh_token_url,omitempty"`
	TenantTokenURL       string   `json:"tenant_access_token_url,omitempty"`
}

type OperationSpec struct {
	ID                 id.ID              `json:"id"`
	TenantID           tenant.ID          `json:"tenant_id"`
	ConnectorID        id.ID              `json:"connector_id,omitempty"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	BusinessDomain     string             `json:"business_domain,omitempty"`
	OwnerTeam          string             `json:"owner_team,omitempty"`
	Type               OperationType      `json:"type"`
	Status             OperationStatus    `json:"status"`
	ImplementationMode ImplementationMode `json:"implementation_mode"`
	BaseURL            string             `json:"base_url"`
	Method             string             `json:"method"`
	Path               string             `json:"path"`
	Headers            map[string]string  `json:"headers,omitempty"`
	Auth               AuthConfig         `json:"auth,omitempty"`
	InputSchema        map[string]any     `json:"input_schema,omitempty"`
	OutputSchema       map[string]any     `json:"output_schema,omitempty"`
	TimeoutMillis      int                `json:"timeout_ms"`
	Version            string             `json:"version,omitempty"`
}

type OperationDependencySummary struct {
	DirectToolCount    int `json:"direct_tool_count"`
	IndirectSkillCount int `json:"indirect_skill_count"`
	IndirectAgentCount int `json:"indirect_agent_count"`
	TotalConsumerCount int `json:"total_consumer_count"`
	BlockingToolCount  int `json:"blocking_tool_count"`
}

type ToolConsumer struct {
	ID             id.ID  `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	RequiresReview bool   `json:"requires_review"`
}

type SkillConsumer struct {
	ID        id.ID  `json:"id"`
	Name      string `json:"name"`
	ViaToolID id.ID  `json:"via_tool_id"`
}

type AgentConsumer struct {
	ID         id.ID  `json:"id"`
	Name       string `json:"name"`
	ViaSkillID id.ID  `json:"via_skill_id"`
}

type OperationDependencies struct {
	OperationID    id.ID                      `json:"operation_id"`
	Summary        OperationDependencySummary `json:"summary"`
	DirectTools    []ToolConsumer             `json:"direct_tools"`
	IndirectSkills []SkillConsumer            `json:"indirect_skills"`
	IndirectAgents []AgentConsumer            `json:"indirect_agents"`
}

type InvokeRequest struct {
	ID          id.ID          `json:"id"`
	TenantID    tenant.ID      `json:"tenant_id"`
	OperationID id.ID          `json:"operation_id"`
	Args        map[string]any `json:"args,omitempty"`
	TraceID     string         `json:"trace_id,omitempty"`
}

type InvokeResult struct {
	RequestID  id.ID          `json:"request_id"`
	Success    bool           `json:"success"`
	Data       map[string]any `json:"data,omitempty"`
	ErrorCode  string         `json:"error_code,omitempty"`
	FinishedAt time.Time      `json:"finished_at"`
}
