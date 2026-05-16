package agentcore

import "flow-anything/core/runtimecontext"

// AgentSpec is the runtime definition consumed by Agent Core.
type AgentSpec struct {
	ID            string
	Name          string
	Description   string
	Prompt        string
	ReasoningMode string
	Model         ModelConfig
	Capabilities  []CapabilityDescriptor
	OutputSchema  []SchemaField
}

// ModelConfig keeps provider-specific model settings outside the reasoning
// strategy implementation.
type ModelConfig struct {
	Provider    string
	Model       string
	Temperature float64
	MaxTokens   int
}

// SchemaField describes capability and agent input/output contracts.
type SchemaField struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Children    []SchemaField
}

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
	ID           string
	Type         string
	Name         string
	Description  string
	InputSchema  []SchemaField
	OutputSchema []SchemaField
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
