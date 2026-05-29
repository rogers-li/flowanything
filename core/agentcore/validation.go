package agentcore

import (
	"fmt"
	"strings"

	"flow-anything/core/jsonutil"
	"flow-anything/core/schema"
)

func validateAgentSpec(agent AgentSpec) error {
	if agent.ID == "" {
		return fmt.Errorf("agent id is required")
	}
	if agent.Policy.MaxIterations < 0 {
		return fmt.Errorf("agent policy max_iterations must be >= 0")
	}
	if agent.Policy.MaxActions < 0 {
		return fmt.Errorf("agent policy max_actions must be >= 0")
	}
	if err := schema.ValidateDefinition(agent.OutputSchema); err != nil {
		return fmt.Errorf("agent output schema is invalid: %w", err)
	}
	for _, capability := range agent.Capabilities {
		if capability.ID == "" {
			return fmt.Errorf("capability id is required")
		}
		if err := schema.ValidateDefinition(capability.InputSchema); err != nil {
			return fmt.Errorf("capability %q input schema is invalid: %w", capability.ID, err)
		}
		if err := schema.ValidateDefinition(capability.OutputSchema); err != nil {
			return fmt.Errorf("capability %q output schema is invalid: %w", capability.ID, err)
		}
	}
	return nil
}

func validateCapabilityInput(action PlannedAction, descriptor CapabilityDescriptor) error {
	if len(descriptor.InputSchema) == 0 {
		return nil
	}
	input := action.Input
	if input == nil {
		input = map[string]any{}
	}
	if err := schema.ValidateValue(descriptor.InputSchema, input); err != nil {
		return fmt.Errorf("action %q input does not match capability schema: %w", action.ID, err)
	}
	return nil
}

func parseAndValidateFinalOutput(agent AgentSpec, content string) (map[string]any, error) {
	if len(agent.OutputSchema) == 0 || !agent.Policy.ValidateFinalOutput {
		return nil, nil
	}
	output, err := parseJSONObject(content)
	if err != nil {
		return nil, fmt.Errorf("final answer is not valid JSON object: %w", err)
	}
	if err := schema.ValidateValue(agent.OutputSchema, output); err != nil {
		return nil, fmt.Errorf("final answer does not match output schema: %w", err)
	}
	return output, nil
}

func finalTextFromOutput(content string, output map[string]any) string {
	for _, key := range []string{"text", "return_message", "answer", "message", "final_answer", "result"} {
		if value, ok := output[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return content
}

func parseJSONObject(content string) (map[string]any, error) {
	var output map[string]any
	if err := jsonutil.UnmarshalObject(content, &output); err != nil {
		return nil, err
	}
	if output == nil {
		return nil, fmt.Errorf("expected JSON object")
	}
	return output, nil
}
