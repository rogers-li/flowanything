package capability

import (
	"context"

	"flow-anything/core/runtimecontext"
	"flow-anything/core/schema"
)

type Kind string

const (
	KindTool      Kind = "tool"
	KindSkill     Kind = "skill"
	KindAgent     Kind = "agent"
	KindWorkflow  Kind = "workflow"
	KindKnowledge Kind = "knowledge"
)

type Descriptor struct {
	ID           string         `json:"id"`
	Kind         Kind           `json:"kind"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	InputSchema  schema.Schema  `json:"input_schema,omitempty"`
	OutputSchema schema.Schema  `json:"output_schema,omitempty"`
	Disabled     bool           `json:"disabled,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type Call struct {
	ID           string                      `json:"id"`
	Kind         Kind                        `json:"kind"`
	Task         string                      `json:"task"`
	Input        map[string]any              `json:"input,omitempty"`
	Reason       string                      `json:"reason,omitempty"`
	TraceID      string                      `json:"trace_id,omitempty"`
	TraceContext runtimecontext.TraceContext `json:"trace_context,omitempty"`
	Metadata     map[string]any              `json:"metadata,omitempty"`
}

type Result struct {
	ID       string         `json:"id"`
	Kind     Kind           `json:"kind"`
	Text     string         `json:"text,omitempty"`
	Output   map[string]any `json:"output,omitempty"`
	Raw      any            `json:"raw,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Invoker interface {
	Invoke(ctx context.Context, call Call) (Result, error)
}

type Capability interface {
	Descriptor() Descriptor
	Invoke(ctx context.Context, call Call) (Result, error)
}

type CapabilityFunc struct {
	Desc Descriptor
	Fn   func(ctx context.Context, call Call) (Result, error)
}

func (c CapabilityFunc) Descriptor() Descriptor {
	return c.Desc
}

func (c CapabilityFunc) Invoke(ctx context.Context, call Call) (Result, error) {
	return c.Fn(ctx, call)
}
