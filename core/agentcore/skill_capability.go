package agentcore

import (
	"fmt"

	"flow-anything/core/runtimecontext"
)

// SkillSpec is the core runtime contract for running a Skill as one callable
// capability. A Skill exposes itself to a parent Agent, while its own
// capabilities remain private to the Skill execution.
type SkillSpec struct {
	ID            string
	Name          string
	Description   string
	Prompt        string
	ReasoningMode string
	Model         ModelConfig
	InputSchema   []SchemaField
	OutputSchema  []SchemaField
	Capabilities  []CapabilityDescriptor
	Policy        AgentPolicy
}

// AgentSpec turns a Skill into the focused Agent runtime used to execute it.
func (s SkillSpec) AgentSpec() AgentSpec {
	mode := s.ReasoningMode
	if mode == "" {
		if len(s.Capabilities) > 0 {
			mode = ReWOOStrategy{}.Name()
		} else {
			mode = DirectStrategy{}.Name()
		}
	}
	return AgentSpec{
		ID:            s.ID,
		Name:          s.Name,
		Description:   s.Description,
		Prompt:        s.Prompt,
		ReasoningMode: mode,
		Model:         s.Model,
		Capabilities:  append([]CapabilityDescriptor(nil), s.Capabilities...),
		OutputSchema:  append([]SchemaField(nil), s.OutputSchema...),
		Policy:        s.Policy,
	}
}

// SkillCapability adapts a SkillSpec into the same capability interface used by
// tools, workflow tools, and sub-agents.
type SkillCapability struct {
	spec   SkillSpec
	runner *Runner
}

func NewSkillCapability(spec SkillSpec, runner *Runner) SkillCapability {
	return SkillCapability{spec: spec, runner: runner}
}

func (c SkillCapability) Descriptor() CapabilityDescriptor {
	return CapabilityDescriptor{
		ID:           c.spec.ID,
		Type:         "skill",
		Name:         c.spec.Name,
		Description:  c.spec.Description,
		InputSchema:  append([]SchemaField(nil), c.spec.InputSchema...),
		OutputSchema: append([]SchemaField(nil), c.spec.OutputSchema...),
	}
}

func (c SkillCapability) Invoke(ctx Context, call CapabilityCall) (CapabilityResult, error) {
	if c.runner == nil {
		return CapabilityResult{}, fmt.Errorf("skill runner is required")
	}
	result, err := c.runner.Run(ctx, AgentRunRequest{
		Agent:        c.spec.AgentSpec(),
		UserMessage:  skillUserMessage(call),
		Context:      call.Input,
		TraceID:      call.TraceID,
		TraceContext: skillChildTraceContext(call.TraceContext),
	})
	if err != nil {
		return CapabilityResult{}, err
	}
	output := result.Output
	if output == nil {
		output = map[string]any{}
	}
	if result.Text != "" {
		output["text"] = result.Text
	}
	return CapabilityResult{
		ID:     c.spec.ID,
		Type:   "skill",
		Text:   result.Text,
		Output: output,
		Raw:    result.Raw,
	}, nil
}

func skillUserMessage(call CapabilityCall) string {
	for _, value := range []string{
		call.Task,
		stringValue(call.Input, "query"),
		stringValue(call.Input, "user_request"),
		stringValue(call.Input, "task"),
		stringValue(call.Input, "message"),
		stringValue(call.Input, "text"),
	} {
		if value != "" {
			return value
		}
	}
	return ""
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func skillChildTraceContext(parent runtimecontext.TraceContext) runtimecontext.TraceContext {
	return runtimecontext.TraceContext{
		TraceID:       parent.TraceID,
		ParentSpanID:  parent.SpanID,
		CorrelationID: parent.CorrelationID,
	}
}
