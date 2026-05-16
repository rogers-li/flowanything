package tool

import (
	"time"

	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type ImplementationType string

const (
	ImplementationConnector ImplementationType = "connector"
	ImplementationKnowledge ImplementationType = "knowledge"
	ImplementationPython    ImplementationType = "python"
	ImplementationMCP       ImplementationType = "mcp"
	ImplementationWorkflow  ImplementationType = "workflow"
)

type SideEffect string

const (
	SideEffectNone  SideEffect = "none"
	SideEffectRead  SideEffect = "read"
	SideEffectWrite SideEffect = "write"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusEnabled  Status = "enabled"
	StatusDisabled Status = "disabled"
)

type ExecutionStatus string

const (
	ExecutionStatusStarted   ExecutionStatus = "started"
	ExecutionStatusSucceeded ExecutionStatus = "succeeded"
	ExecutionStatusFailed    ExecutionStatus = "failed"
)

type RetryPolicy struct {
	MaxAttempts   int `json:"max_attempts"`
	BackoffMillis int `json:"backoff_ms"`
}

type Spec struct {
	ID                   id.ID              `json:"id"`
	TenantID             tenant.ID          `json:"tenant_id"`
	Name                 string             `json:"name"`
	Description          string             `json:"description"`
	BusinessDomain       string             `json:"business_domain,omitempty"`
	OwnerTeam            string             `json:"owner_team,omitempty"`
	Status               Status             `json:"status"`
	LLMDescription       string             `json:"llm_description,omitempty"`
	Implementation       ImplementationType `json:"implementation"`
	Binding              Binding            `json:"binding"`
	InputSchema          map[string]any     `json:"input_schema,omitempty"`
	OutputSchema         map[string]any     `json:"output_schema,omitempty"`
	SideEffect           SideEffect         `json:"side_effect"`
	RiskLevel            RiskLevel          `json:"risk_level"`
	RequiresConfirmation bool               `json:"requires_confirmation"`
	TimeoutMillis        int                `json:"timeout_ms"`
	RetryPolicy          RetryPolicy        `json:"retry_policy,omitempty"`
	Version              string             `json:"version"`
}

func (s Spec) RequiresExecutionConfirmation() bool {
	return s.RequiresConfirmation || s.RiskLevel == RiskHigh
}

type Binding struct {
	ConnectorOperationID id.ID             `json:"connector_operation_id,omitempty"`
	KnowledgeBaseIDs     []id.ID           `json:"knowledge_base_ids,omitempty"`
	PythonPackageID      id.ID             `json:"python_package_id,omitempty"`
	MCPServerID          id.ID             `json:"mcp_server_id,omitempty"`
	MCPServerURL         string            `json:"mcp_server_url,omitempty"`
	MCPTransport         string            `json:"mcp_transport,omitempty"`
	MCPHeaders           map[string]string `json:"mcp_headers,omitempty"`
	MCPToolName          string            `json:"mcp_tool_name,omitempty"`
	WorkflowID           id.ID             `json:"workflow_id,omitempty"`
}

type BackendResult struct {
	RequestID        id.ID              `json:"request_id"`
	Success          bool               `json:"success"`
	Data             map[string]any     `json:"data,omitempty"`
	ErrorCode        string             `json:"error_code,omitempty"`
	ErrorReason      string             `json:"error_reason,omitempty"`
	StartedAt        time.Time          `json:"started_at,omitempty"`
	FinishedAt       time.Time          `json:"finished_at,omitempty"`
	WorkflowRun      *workflow.Run      `json:"workflow_run,omitempty"`
	WorkflowNodeRuns []workflow.NodeRun `json:"workflow_node_runs,omitempty"`
}

type PythonRunRequest struct {
	ID        id.ID          `json:"id"`
	TenantID  tenant.ID      `json:"tenant_id"`
	ToolID    id.ID          `json:"tool_id"`
	PackageID id.ID          `json:"package_id"`
	Args      map[string]any `json:"args,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
}

type MCPCallRequest struct {
	ID        id.ID             `json:"id"`
	TenantID  tenant.ID         `json:"tenant_id"`
	ToolID    id.ID             `json:"tool_id"`
	ServerID  id.ID             `json:"server_id"`
	ServerURL string            `json:"server_url,omitempty"`
	Transport string            `json:"transport,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	ToolName  string            `json:"tool_name"`
	Args      map[string]any    `json:"args,omitempty"`
	TraceID   string            `json:"trace_id,omitempty"`
}

type WorkflowRunRequest struct {
	ID         id.ID          `json:"id"`
	TenantID   tenant.ID      `json:"tenant_id"`
	ToolID     id.ID          `json:"tool_id"`
	WorkflowID id.ID          `json:"workflow_id"`
	Args       map[string]any `json:"args,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
}

type DependencySummary struct {
	DirectSkillCount   int `json:"direct_skill_count"`
	DirectAgentCount   int `json:"direct_agent_count"`
	IndirectAgentCount int `json:"indirect_agent_count"`
	TotalConsumerCount int `json:"total_consumer_count"`
}

type SkillConsumer struct {
	ID     id.ID  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

type AgentConsumer struct {
	ID         id.ID  `json:"id"`
	Name       string `json:"name"`
	ViaSkillID id.ID  `json:"via_skill_id,omitempty"`
	Source     string `json:"source,omitempty"`
}

type Dependencies struct {
	ToolID         id.ID             `json:"tool_id"`
	Summary        DependencySummary `json:"summary"`
	DirectSkills   []SkillConsumer   `json:"direct_skills"`
	DirectAgents   []AgentConsumer   `json:"direct_agents"`
	IndirectAgents []AgentConsumer   `json:"indirect_agents"`
}

type Call struct {
	ID             id.ID              `json:"id"`
	TenantID       tenant.ID          `json:"tenant_id"`
	ToolID         id.ID              `json:"tool_id"`
	Name           string             `json:"name"`
	Implementation ImplementationType `json:"implementation"`
	Args           map[string]any     `json:"args,omitempty"`
	Confirmed      bool               `json:"confirmed,omitempty"`
	TraceID        string             `json:"trace_id,omitempty"`
}

type Result struct {
	CallID           id.ID              `json:"call_id"`
	ToolID           id.ID              `json:"tool_id"`
	Success          bool               `json:"success"`
	Data             map[string]any     `json:"data,omitempty"`
	ErrorCode        string             `json:"error_code,omitempty"`
	ErrorReason      string             `json:"error_reason,omitempty"`
	StartedAt        time.Time          `json:"started_at"`
	FinishedAt       time.Time          `json:"finished_at"`
	WorkflowRun      *workflow.Run      `json:"workflow_run,omitempty"`
	WorkflowNodeRuns []workflow.NodeRun `json:"workflow_node_runs,omitempty"`
}

type ExecutionRecord struct {
	CallID               id.ID              `json:"call_id"`
	TenantID             tenant.ID          `json:"tenant_id"`
	ToolID               id.ID              `json:"tool_id"`
	ToolName             string             `json:"tool_name,omitempty"`
	Implementation       ImplementationType `json:"implementation,omitempty"`
	RiskLevel            RiskLevel          `json:"risk_level,omitempty"`
	RequiresConfirmation bool               `json:"requires_confirmation"`
	Confirmed            bool               `json:"confirmed"`
	SpecVersion          string             `json:"spec_version,omitempty"`
	TraceID              string             `json:"trace_id,omitempty"`
	ArgsSummary          map[string]any     `json:"args_summary,omitempty"`
	Status               ExecutionStatus    `json:"status"`
	Result               *Result            `json:"result,omitempty"`
	ErrorCode            string             `json:"error_code,omitempty"`
	ErrorReason          string             `json:"error_reason,omitempty"`
	StartedAt            time.Time          `json:"started_at"`
	FinishedAt           time.Time          `json:"finished_at,omitempty"`
	DurationMillis       int64              `json:"duration_ms,omitempty"`
}
