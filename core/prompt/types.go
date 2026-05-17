package prompt

import "flow-anything/core/schema"

type Spec struct {
	System    string            `json:"system"`
	Developer string            `json:"developer,omitempty"`
	Templates map[string]string `json:"templates,omitempty"`
	Variables schema.Schema     `json:"variables,omitempty"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type PlanningPromptRequest struct {
	AgentName        string
	AgentDescription string
	Base             Spec
	Capabilities     []CapabilityDescriptor
	OutputSchema     schema.Schema
	UserRequestLabel string
}

type CapabilityDescriptor struct {
	ID           string
	Kind         string
	Name         string
	Description  string
	InputSchema  schema.Schema
	OutputSchema schema.Schema
}
