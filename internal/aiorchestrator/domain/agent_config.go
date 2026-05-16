package domain

import (
	"strings"

	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
)

type AgentConfig struct {
	Agent  agent.Profile
	Skills []skill.Spec
	Tools  []tool.Spec
}

// SystemPrompt returns the prompt that should be sent as the model system
// message. A configured Agent prompt is treated as the authored source of truth
// and is not wrapped with platform copy, so prompt authors can reason about the
// exact instruction seen by the model.
func (c AgentConfig) SystemPrompt(basePrompt string) string {
	if prompt := strings.TrimSpace(c.Agent.SystemPrompt); prompt != "" {
		return prompt
	}

	var builder strings.Builder
	if strings.TrimSpace(basePrompt) != "" {
		builder.WriteString(strings.TrimSpace(basePrompt))
	}
	if c.Agent.Name != "" {
		writePromptLine(&builder, "当前 Agent: "+c.Agent.Name)
	}
	if c.Agent.Description != "" {
		writePromptLine(&builder, "Agent 描述: "+c.Agent.Description)
	}
	for _, spec := range c.Skills {
		if spec.SystemPrompt == "" {
			continue
		}
		writePromptLine(&builder, "Skill "+spec.Name+": "+spec.SystemPrompt)
	}

	return builder.String()
}

func writePromptLine(builder *strings.Builder, line string) {
	if builder.Len() > 0 {
		builder.WriteByte('\n')
	}
	builder.WriteString(line)
}
