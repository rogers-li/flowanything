package tools

import (
	"time"

	"flow-anything/core/runtimecontext"
)

// ToolSpec is the standard configuration protocol for one callable tool.
type ToolSpec struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Type           ToolType           `json:"type"`
	InputSchema    []SchemaField      `json:"input_schema"`
	OutputSchema   []SchemaField      `json:"output_schema"`
	Implementation ToolImplementation `json:"implementation"`
	Policy         ToolPolicy         `json:"policy"`
	Metadata       map[string]any     `json:"metadata"`
	Enabled        bool               `json:"enabled"`
}

// ToolType describes the product-facing implementation category.
type ToolType string

const (
	ToolTypeNative    ToolType = "native"
	ToolTypeConnector ToolType = "connector"
	ToolTypeWorkflow  ToolType = "workflow"
	ToolTypeMCP       ToolType = "mcp"
	ToolTypeScript    ToolType = "script"
)

// ToolImplementation is intentionally generic. The concrete executor for
// Implementation.Kind owns the Config protocol.
type ToolImplementation struct {
	Kind   string         `json:"kind"`
	Ref    string         `json:"ref"`
	Config map[string]any `json:"config"`
}

type ToolPolicy struct {
	Timeout       time.Duration `json:"timeout"`
	RequireReview bool          `json:"require_review"`
	RetryPolicy   RetryPolicy   `json:"retry_policy"`
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

// ToolCall is the runtime request to execute a tool.
type ToolCall struct {
	CallID       string                      `json:"call_id"`
	ToolID       string                      `json:"tool_id"`
	Input        map[string]any              `json:"input"`
	Metadata     map[string]any              `json:"metadata"`
	TraceID      string                      `json:"trace_id"`
	TraceContext runtimecontext.TraceContext `json:"trace_context"`
}

// ToolResult is the runtime response from a tool execution.
type ToolResult struct {
	CallID     string         `json:"call_id"`
	ToolID     string         `json:"tool_id"`
	Success    bool           `json:"success"`
	Output     map[string]any `json:"output"`
	Raw        any            `json:"raw"`
	Error      ToolError      `json:"error"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
}

type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
