package domain

import (
	"time"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

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

type FlowRun struct {
	ID          id.ID          `json:"id"`
	TenantID    tenant.ID      `json:"tenant_id"`
	FlowID      id.ID          `json:"flow_id"`
	FlowVersion string         `json:"flow_version,omitempty"`
	Status      RunStatus      `json:"status"`
	Input       map[string]any `json:"input,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
}

type NodeRun struct {
	ID         id.ID          `json:"id"`
	TenantID   tenant.ID      `json:"tenant_id"`
	RunID      id.ID          `json:"run_id"`
	FlowID     id.ID          `json:"flow_id"`
	NodeID     id.ID          `json:"node_id"`
	NodeType   NodeType       `json:"node_type"`
	NodeName   string         `json:"node_name,omitempty"`
	Status     NodeRunStatus  `json:"status"`
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

type NodeResult struct {
	Output      map[string]any `json:"output,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
	NextNodeIDs []id.ID        `json:"next_node_ids,omitempty"`
	Stop        bool           `json:"stop,omitempty"`
}
