package model

import (
	"time"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ChatRequest struct {
	ID         id.ID            `json:"id"`
	TenantID   tenant.ID        `json:"tenant_id"`
	TraceID    string           `json:"trace_id,omitempty"`
	Model      string           `json:"model,omitempty"`
	Messages   []Message        `json:"messages"`
	Tools      []ToolDefinition `json:"tools,omitempty"`
	ToolChoice string           `json:"tool_choice,omitempty"`
	Options    Options          `json:"options,omitempty"`
}

type Options struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	ID           id.ID     `json:"id"`
	RequestID    id.ID     `json:"request_id"`
	TraceID      string    `json:"trace_id,omitempty"`
	Provider     string    `json:"provider,omitempty"`
	ProviderURL  string    `json:"provider_url,omitempty"`
	Model        string    `json:"model"`
	Message      Message   `json:"message"`
	FinishReason string    `json:"finish_reason"`
	Usage        Usage     `json:"usage,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}
