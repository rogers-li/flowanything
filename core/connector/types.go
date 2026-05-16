package connector

import (
	"time"

	"flow-anything/core/runtimecontext"
)

// ConnectorSpec describes how to communicate with one external system.
type ConnectorSpec struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Protocol    ProtocolSpec   `json:"protocol"`
	Auth        AuthSpec       `json:"auth"`
	Metadata    map[string]any `json:"metadata"`
	Enabled     bool           `json:"enabled"`
}

// ProtocolSpec selects the protocol adapter and stores protocol-level config.
type ProtocolSpec struct {
	Kind    string         `json:"kind"`
	BaseURL string         `json:"base_url"`
	Config  map[string]any `json:"config"`
}

// AuthSpec keeps auth configuration declarative. Runtime adapters can resolve
// actual credentials from environment variables, secret managers, or token
// providers outside Connector Core.
type AuthSpec struct {
	Type      string         `json:"type"`
	SecretRef string         `json:"secret_ref"`
	Config    map[string]any `json:"config"`
}

// OperationSpec describes one callable API/action under a Connector.
type OperationSpec struct {
	ID           string            `json:"id"`
	ConnectorID  string            `json:"connector_id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	InputSchema  []SchemaField     `json:"input_schema"`
	OutputSchema []SchemaField     `json:"output_schema"`
	Request      OperationRequest  `json:"request"`
	Response     OperationResponse `json:"response"`
	Policy       OperationPolicy   `json:"policy"`
	Metadata     map[string]any    `json:"metadata"`
	Enabled      bool              `json:"enabled"`
}

type OperationRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Config  map[string]any    `json:"config"`
}

type OperationResponse struct {
	SuccessStatusCodes []int          `json:"success_status_codes"`
	Config             map[string]any `json:"config"`
}

type OperationPolicy struct {
	Timeout     time.Duration `json:"timeout"`
	RetryPolicy RetryPolicy   `json:"retry_policy"`
}

type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	Backoff     time.Duration `json:"backoff"`
}

type SchemaField struct {
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Description string        `json:"description"`
	Required    bool          `json:"required"`
	Children    []SchemaField `json:"children"`
}

// InvokeRequest is a runtime operation invocation.
type InvokeRequest struct {
	CallID       string                      `json:"call_id"`
	OperationID  string                      `json:"operation_id"`
	Input        map[string]any              `json:"input"`
	Metadata     map[string]any              `json:"metadata"`
	TraceID      string                      `json:"trace_id"`
	TraceContext runtimecontext.TraceContext `json:"trace_context"`
}

type InvokeResult struct {
	CallID      string         `json:"call_id"`
	ConnectorID string         `json:"connector_id"`
	OperationID string         `json:"operation_id"`
	Success     bool           `json:"success"`
	Output      map[string]any `json:"output"`
	Raw         any            `json:"raw"`
	Error       ConnectorError `json:"error"`
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  time.Time      `json:"finished_at"`
}

type ConnectorError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
