package agentcore

import (
	"flow-anything/core/runtimecontext"
	"flow-anything/core/schema"
)

// AgentSpec is the runtime definition consumed by Agent Core.
type AgentSpec struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Prompt        string                 `json:"prompt"`
	ReasoningMode string                 `json:"reasoning_mode"`
	Model         ModelConfig            `json:"model"`
	Capabilities  []CapabilityDescriptor `json:"capabilities"`
	OutputSchema  []SchemaField          `json:"output_schema"`
	Policy        AgentPolicy            `json:"policy"`
}

// ModelConfig keeps provider-specific model settings outside the reasoning
// strategy implementation.
type ModelConfig struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// AgentPolicy keeps agent execution bounded and observable. Zero values are
// normalized by the runner so configs can stay compact.
type AgentPolicy struct {
	MaxIterations       int  `json:"max_iterations"`
	MaxActions          int  `json:"max_actions"`
	ValidateFinalOutput bool `json:"validate_final_output"`
	MaxContextTokens    int  `json:"max_context_tokens"`
	MaxHistoryMessages  int  `json:"max_history_messages"`
	MaxMemoryItems      int  `json:"max_memory_items"`
	MaxMessageChars     int  `json:"max_message_chars"`
}

// SchemaField describes capability and agent input/output contracts. The
// concrete schema semantics live in core/schema.
type SchemaField = schema.Field

// Message is a provider-neutral chat message.
type Message struct {
	Role    string
	Content string
}

// ModelRequest is passed to a ModelClient.
type ModelRequest struct {
	Model        ModelConfig
	Messages     []Message
	Tools        []CapabilityDescriptor
	Metadata     map[string]any
	TraceID      string
	TraceContext runtimecontext.TraceContext
}

// ModelResponse is returned by a ModelClient.
type ModelResponse struct {
	Message  Message
	Raw      map[string]any
	Usage    map[string]any
	TraceID  string
	Provider string
	Model    string
}

// ModelClient is the only LLM dependency required by Agent Core.
type ModelClient interface {
	Chat(ctx Context, req ModelRequest) (ModelResponse, error)
}

// Context aliases the standard context interface used by Agent Core. The alias
// keeps public signatures compact while staying compatible with context.Context.
type Context interface {
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

// CapabilityDescriptor is what the LLM sees.
type CapabilityDescriptor struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	InputSchema  []SchemaField `json:"input_schema"`
	OutputSchema []SchemaField `json:"output_schema"`
}

// Capability is the common abstraction for tools, skills, sub-agents, workflow
// tools, and future retrieval capabilities.
type Capability interface {
	Descriptor() CapabilityDescriptor
	Invoke(ctx Context, call CapabilityCall) (CapabilityResult, error)
}

// CapabilityCall is one planned action invocation.
type CapabilityCall struct {
	ID           string
	Type         string
	Task         string
	Input        map[string]any
	Reason       string
	TraceID      string
	TraceContext runtimecontext.TraceContext
}

// CapabilityResult is the capability execution output.
type CapabilityResult struct {
	ID     string
	Type   string
	Text   string
	Output map[string]any
	Raw    any
}

// AgentRunRequest is passed to Runner.
type AgentRunRequest struct {
	Agent        AgentSpec
	UserMessage  string
	Conversation []Message
	Context      map[string]any
	TraceID      string
	TraceContext runtimecontext.TraceContext
}

// AgentRunResult is returned by Runner.
type AgentRunResult struct {
	Text    string
	Output  map[string]any
	Actions []ActionResult
	Raw     any
}

// PlannedAction is the model-readable action protocol.
type PlannedAction struct {
	Type   string         `json:"type"`
	ID     string         `json:"id"`
	Task   string         `json:"task"`
	Input  map[string]any `json:"input"`
	Reason string         `json:"reason"`
}

// ActionResult captures one action and its observation.
type ActionResult struct {
	Action PlannedAction
	Result CapabilityResult
	Error  string
}
