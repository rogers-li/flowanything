package flowengine

import (
	"strings"

	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type NodeResult struct {
	Output         map[string]any `json:"output,omitempty"`
	ContextWrites  map[string]any `json:"context_writes,omitempty"`
	ResponseWrites map[string]any `json:"response_writes,omitempty"`
	NextNodeIDs    []id.ID        `json:"next_node_ids,omitempty"`
	Stop           bool           `json:"stop,omitempty"`
}

func buildNodeInput(node workflow.Node, ctx RunContext) (map[string]any, error) {
	mapping, ok := mapConfig(node.Config, "input_mapping")
	if !ok {
		mapping, ok = mapConfig(node.Config, "input_mappings")
	}
	if !ok {
		return cloneMap(ctx.Input), nil
	}

	input := make(map[string]any, len(mapping))
	for target, source := range mapping {
		value, err := evaluateConfigValue(source, ctx, nil, nil)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to resolve workflow node input mapping", err)
		}
		input[target] = value
	}
	return input, nil
}

func applyOutputMappings(node workflow.Node, ctx RunContext, rawOutput map[string]any) (NodeResult, error) {
	mappingOutput := outputForMappings(node, rawOutput)
	output := cloneMap(mappingOutput)
	if supportsOutputMapping(node.Type) {
		if mapping, ok := mapConfig(node.Config, "output_mapping"); ok && len(mapping) > 0 {
			output = map[string]any{}
			for target, source := range mapping {
				value, err := evaluateConfigValue(source, ctx, mappingOutput, nil)
				if err != nil {
					return NodeResult{}, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to resolve workflow node output mapping", err)
				}
				output[target] = value
			}
		}
	}

	// Agent nodes already produce an output object according to output_schema.
	// The flow engine should not transform that object again with output_mapping;
	// it only writes selected fields into shared context via write_context.
	if node.Type == workflow.NodeTypeAgent {
		output = cloneMap(mappingOutput)
	}

	writes := map[string]any{}
	for _, key := range []string{"write_context", "context_writes"} {
		mapping, ok := mapConfig(node.Config, key)
		if !ok {
			continue
		}
		for target, source := range mapping {
			value, err := evaluateConfigValue(source, ctx, mappingOutput, output)
			if err != nil {
				return NodeResult{}, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to resolve workflow context write", err)
			}
			writes[target] = value
		}
	}

	responseWrites := automaticResponseWrites(node, mappingOutput)

	return NodeResult{Output: output, ContextWrites: writes, ResponseWrites: responseWrites}, nil
}

func supportsOutputMapping(nodeType workflow.NodeType) bool {
	switch nodeType {
	case workflow.NodeTypeAgent:
		return false
	default:
		return true
	}
}

func outputForMappings(node workflow.Node, rawOutput map[string]any) map[string]any {
	if node.Type != workflow.NodeTypeConnectorOperation && node.Type != workflow.NodeTypeTool {
		return rawOutput
	}
	data, ok := rawOutput["data"]
	if !ok {
		return rawOutput
	}
	payload, ok := data.(map[string]any)
	if !ok {
		return map[string]any{"value": cloneValue(data)}
	}
	output := cloneMap(payload)
	if _, exists := output["data"]; !exists {
		output["data"] = cloneValue(data)
	}
	for _, key := range []string{"request_id", "call_id", "tool_id", "success", "error_code", "error_reason", "started_at", "finished_at"} {
		if _, exists := output[key]; exists {
			continue
		}
		if value, exists := rawOutput[key]; exists {
			output[key] = cloneValue(value)
		}
	}
	return output
}

func automaticResponseWrites(node workflow.Node, rawOutput map[string]any) map[string]any {
	if node.Type != workflow.NodeTypeConnectorOperation && node.Type != workflow.NodeTypeTool {
		return nil
	}
	alias := strings.TrimSpace(stringFromConfig(node.Config, "response_alias"))
	if alias == "" {
		switch node.Type {
		case workflow.NodeTypeConnectorOperation:
			alias = firstNonEmptyConfig(node.Config, "connector_operation_id", "operation_id")
		case workflow.NodeTypeTool:
			alias = firstNonEmptyConfig(node.Config, "tool_id")
		}
	}
	if alias == "" {
		alias = node.ID.String()
	}

	domain := "tool"
	if node.Type == workflow.NodeTypeConnectorOperation {
		domain = "connector"
	}
	return map[string]any{
		"responses." + domain + "." + alias: cloneMap(rawOutput),
	}
}

func evaluateConfigValue(source any, ctx RunContext, rawOutput map[string]any, output map[string]any) (any, error) {
	text, ok := source.(string)
	if !ok {
		return cloneValue(source), nil
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, "$") {
		return text, nil
	}
	switch {
	case strings.HasPrefix(trimmed, "$."):
		value, ok := readJSONLikePath(rawOutput, strings.TrimPrefix(trimmed, "$."))
		if !ok {
			return nil, nil
		}
		return value, nil
	case strings.HasPrefix(trimmed, "$output."):
		value, ok := readJSONLikePath(output, strings.TrimPrefix(trimmed, "$output."))
		if !ok {
			return nil, nil
		}
		return value, nil
	default:
		value, ok := ctx.Read(trimmed)
		if !ok {
			return nil, nil
		}
		return value, nil
	}
}

func readJSONLikePath(root map[string]any, path string) (any, bool) {
	if root == nil {
		return nil, false
	}
	if strings.TrimSpace(path) == "" {
		return root, true
	}
	parts := strings.Split(path, ".")
	var current any = root
	return readParts(current, parts)
}

func mapConfig(config map[string]any, key string) (map[string]any, bool) {
	if config == nil {
		return nil, false
	}
	value, ok := config[key]
	if !ok {
		return nil, false
	}
	typed, ok := value.(map[string]any)
	if ok {
		return typed, true
	}
	return nil, false
}

func firstNonEmptyConfig(config map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromConfig(config, key); value != "" {
			return value
		}
	}
	return ""
}

func stringFromConfig(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
