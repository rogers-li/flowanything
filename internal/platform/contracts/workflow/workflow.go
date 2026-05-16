package workflow

import (
	"time"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusEnabled  Status = "enabled"
	StatusDisabled Status = "disabled"
)

type Profile string

const (
	ProfileToolWorkflow  Profile = "tool_workflow"
	ProfileAgentWorkflow Profile = "agent_workflow"
)

type NodeType string

const (
	NodeTypeStart              NodeType = "start"
	NodeTypeEnd                NodeType = "end"
	NodeTypeJoin               NodeType = "join"
	NodeTypeTransform          NodeType = "transform"
	NodeTypeCondition          NodeType = "condition"
	NodeTypeConnectorOperation NodeType = "connector_operation"
	NodeTypeTool               NodeType = "tool"
	NodeTypeSkill              NodeType = "skill"
	NodeTypeAgent              NodeType = "agent"
)

type EdgeType string

const (
	EdgeTypeDefault     EdgeType = "default"
	EdgeTypeConditional EdgeType = "conditional"
	EdgeTypeFallback    EdgeType = "fallback"
)

type Spec struct {
	ID             id.ID           `json:"id"`
	TenantID       tenant.ID       `json:"tenant_id"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	BusinessDomain string          `json:"business_domain,omitempty"`
	OwnerTeam      string          `json:"owner_team,omitempty"`
	Status         Status          `json:"status"`
	Profile        Profile         `json:"profile"`
	ContextSchema  map[string]any  `json:"context_schema,omitempty"`
	InputSchema    map[string]any  `json:"input_schema,omitempty"`
	OutputSchema   map[string]any  `json:"output_schema,omitempty"`
	Graph          Graph           `json:"graph"`
	Policy         ExecutionPolicy `json:"policy,omitempty"`
	UI             map[string]any  `json:"ui,omitempty"`
	Version        string          `json:"version"`
}

type Graph struct {
	EntryNodeID id.ID          `json:"entry_node_id"`
	Nodes       map[id.ID]Node `json:"nodes"`
	Edges       []Edge         `json:"edges,omitempty"`
}

type Node struct {
	ID            id.ID          `json:"id"`
	Type          NodeType       `json:"type"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Position      Position       `json:"position,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	TimeoutMillis int            `json:"timeout_ms,omitempty"`
	RetryPolicy   RetryPolicy    `json:"retry_policy,omitempty"`
}

type Position struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type Edge struct {
	ID          id.ID          `json:"id"`
	FromNodeID  id.ID          `json:"from_node_id"`
	ToNodeID    id.ID          `json:"to_node_id"`
	Type        EdgeType       `json:"type"`
	Condition   *EdgeCondition `json:"condition,omitempty"`
	Description string         `json:"description,omitempty"`
}

type EdgeCondition struct {
	Path   string `json:"path"`
	Equals any    `json:"equals,omitempty"`
	Exists *bool  `json:"exists,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts   int `json:"max_attempts,omitempty"`
	BackoffMillis int `json:"backoff_ms,omitempty"`
}

type ExecutionPolicy struct {
	MaxSteps       int `json:"max_steps,omitempty"`
	MaxParallelism int `json:"max_parallelism,omitempty"`
	TimeoutMillis  int `json:"timeout_ms,omitempty"`
}

type RunRequest struct {
	ID         id.ID          `json:"id,omitempty"`
	TenantID   tenant.ID      `json:"tenant_id"`
	WorkflowID id.ID          `json:"workflow_id"`
	Workflow   *Spec          `json:"workflow,omitempty"`
	Input      map[string]any `json:"input,omitempty"`
	Context    map[string]any `json:"context,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
}

type RunResponse struct {
	Run      Run       `json:"run"`
	NodeRuns []NodeRun `json:"node_runs,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type RunListResponse struct {
	Items []Run `json:"items"`
}

type ReplayRunRequest struct {
	TenantID tenant.ID      `json:"tenant_id"`
	RunID    id.ID          `json:"run_id,omitempty"`
	Input    map[string]any `json:"input,omitempty"`
	Context  map[string]any `json:"context,omitempty"`
	TraceID  string         `json:"trace_id,omitempty"`
}

type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

type NodeRunStatus string

const (
	NodeRunStatusPending   NodeRunStatus = "pending"
	NodeRunStatusRunning   NodeRunStatus = "running"
	NodeRunStatusSucceeded NodeRunStatus = "succeeded"
	NodeRunStatusFailed    NodeRunStatus = "failed"
	NodeRunStatusSkipped   NodeRunStatus = "skipped"
	NodeRunStatusCanceled  NodeRunStatus = "canceled"
)

type Run struct {
	ID         id.ID          `json:"id"`
	TenantID   tenant.ID      `json:"tenant_id"`
	WorkflowID id.ID          `json:"workflow_id"`
	Version    string         `json:"version,omitempty"`
	Status     RunStatus      `json:"status"`
	Input      map[string]any `json:"input,omitempty"`
	Context    map[string]any `json:"context,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

type NodeRun struct {
	ID         id.ID          `json:"id"`
	TenantID   tenant.ID      `json:"tenant_id"`
	RunID      id.ID          `json:"run_id"`
	WorkflowID id.ID          `json:"workflow_id"`
	NodeID     id.ID          `json:"node_id"`
	NodeType   NodeType       `json:"node_type"`
	NodeName   string         `json:"node_name,omitempty"`
	Status     NodeRunStatus  `json:"status"`
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Context    map[string]any `json:"context,omitempty"`
	Error      string         `json:"error,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}
