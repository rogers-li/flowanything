package domain

import (
	"time"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type TraceStatus string

const (
	TraceStatusRunning   TraceStatus = "running"
	TraceStatusSucceeded TraceStatus = "succeeded"
	TraceStatusFailed    TraceStatus = "failed"
)

type TraceStepType string

const (
	TraceStepEvent     TraceStepType = "event"
	TraceStepAgent     TraceStepType = "agent"
	TraceStepSkill     TraceStepType = "skill"
	TraceStepModel     TraceStepType = "model"
	TraceStepContext   TraceStepType = "context"
	TraceStepTool      TraceStepType = "tool"
	TraceStepWorkflow  TraceStepType = "workflow"
	TraceStepConnector TraceStepType = "connector"
)

type TraceStepStatus string

const (
	TraceStepStatusStarted   TraceStepStatus = "started"
	TraceStepStatusSucceeded TraceStepStatus = "succeeded"
	TraceStepStatusFailed    TraceStepStatus = "failed"
	TraceStepStatusSkipped   TraceStepStatus = "skipped"
)

type TraceRecord struct {
	TraceID        string      `json:"trace_id"`
	TenantID       tenant.ID   `json:"tenant_id"`
	AgentID        id.ID       `json:"agent_id,omitempty"`
	SessionID      id.ID       `json:"session_id,omitempty"`
	EventID        id.ID       `json:"event_id,omitempty"`
	Status         TraceStatus `json:"status"`
	StartedAt      time.Time   `json:"started_at"`
	FinishedAt     time.Time   `json:"finished_at,omitempty"`
	DurationMillis int64       `json:"duration_ms,omitempty"`
	Error          string      `json:"error,omitempty"`
	Steps          []TraceStep `json:"steps"`
}

type TraceStep struct {
	ID             string          `json:"id"`
	ParentID       string          `json:"parent_id,omitempty"`
	Type           TraceStepType   `json:"type"`
	Name           string          `json:"name"`
	Status         TraceStepStatus `json:"status"`
	StartedAt      time.Time       `json:"started_at"`
	FinishedAt     time.Time       `json:"finished_at,omitempty"`
	DurationMillis int64           `json:"duration_ms,omitempty"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
	Error          string          `json:"error,omitempty"`
}

func NewTraceRecord(traceID string, tenantID tenant.ID, agentID, sessionID, eventID id.ID, now time.Time) TraceRecord {
	return TraceRecord{
		TraceID:   traceID,
		TenantID:  tenantID,
		AgentID:   agentID,
		SessionID: sessionID,
		EventID:   eventID,
		Status:    TraceStatusRunning,
		StartedAt: now.UTC(),
		Steps:     []TraceStep{},
	}
}

func NewTraceStep(stepType TraceStepType, name string, status TraceStepStatus, startedAt, finishedAt time.Time, metadata map[string]any) TraceStep {
	step := TraceStep{
		ID:         id.New("trstep").String(),
		Type:       stepType,
		Name:       name,
		Status:     status,
		StartedAt:  startedAt.UTC(),
		FinishedAt: finishedAt.UTC(),
		Metadata:   metadata,
	}
	if !startedAt.IsZero() && !finishedAt.IsZero() {
		step.DurationMillis = finishedAt.Sub(startedAt).Milliseconds()
	}
	return step
}
