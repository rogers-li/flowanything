package domain

import (
	"strings"

	"flow-anything/internal/platform/kernel/id"
)

type RunContext struct {
	Input       map[string]any           `json:"input,omitempty"`
	Variables   map[string]any           `json:"variables,omitempty"`
	NodeOutputs map[id.ID]map[string]any `json:"node_outputs,omitempty"`
}

func NewRunContext(input map[string]any) RunContext {
	return RunContext{
		Input:       cloneMap(input),
		Variables:   map[string]any{},
		NodeOutputs: map[id.ID]map[string]any{},
	}
}

func (c RunContext) WithNodeResult(nodeID id.ID, result NodeResult) RunContext {
	next := RunContext{
		Input:       cloneMap(c.Input),
		Variables:   cloneMap(c.Variables),
		NodeOutputs: cloneNestedMap(c.NodeOutputs),
	}
	if result.Output != nil {
		next.NodeOutputs[nodeID] = cloneMap(result.Output)
	}
	for key, value := range result.Variables {
		next.Variables[key] = value
	}
	return next
}

func (c RunContext) ValueAtPath(path string) (any, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, false
	}

	var current any
	switch strings.ToLower(parts[0]) {
	case "input":
		current = c.Input
	case "variables", "runtime":
		current = c.Variables
	case "nodes", "node_outputs":
		if len(parts) < 2 {
			return nil, false
		}
		nodeOutput, ok := c.NodeOutputs[id.ID(parts[1])]
		if !ok {
			return nil, false
		}
		current = nodeOutput
		parts = append(parts[:1], parts[2:]...)
	default:
		return nil, false
	}

	for _, part := range parts[1:] {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = asMap[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneNestedMap(source map[id.ID]map[string]any) map[id.ID]map[string]any {
	result := make(map[id.ID]map[string]any, len(source))
	for key, value := range source {
		result[key] = cloneMap(value)
	}
	return result
}
