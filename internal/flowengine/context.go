package flowengine

import (
	"fmt"
	"strings"

	"flow-anything/internal/platform/kernel/id"
)

type RunContext struct {
	Input map[string]any `json:"input,omitempty"`
	Ctx   map[string]any `json:"ctx,omitempty"`
	Vars  map[string]any `json:"vars,omitempty"`
	Last  map[string]any `json:"last,omitempty"`
}

func NewRunContext(input map[string]any, initialContext map[string]any) RunContext {
	ctx := normalizeInitialContext(input, initialContext)
	return RunContext{
		Input: cloneMap(input),
		Ctx:   ctx,
		Vars:  map[string]any{},
		Last:  map[string]any{},
	}
}

// WithNodeResult applies a node's declared context writes, stores runtime-owned
// response archives, and records a compact last-node snapshot.
func (c RunContext) WithNodeResult(nodeID id.ID, result NodeResult) RunContext {
	next := RunContext{
		Input: cloneMap(c.Input),
		Ctx:   cloneMap(c.Ctx),
		Vars:  cloneMap(c.Vars),
		Last: map[string]any{
			"node_id": nodeID.String(),
			"output":  cloneMap(result.Output),
		},
	}
	for path, value := range result.ContextWrites {
		_ = next.Write(path, value)
	}
	for path, value := range result.ResponseWrites {
		_ = next.writeSystem(path, value)
	}
	return next
}

func (c RunContext) Read(path string) (any, bool) {
	path = normalizePath(path)
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, false
	}

	var current any
	switch strings.ToLower(parts[0]) {
	case "input", "flow_input":
		current = c.Input
	case "ctx", "context":
		current = c.Ctx
	case "flow_output", "output":
		current = domainMap(c.Ctx, "flow_output")
	case "responses", "response":
		current = domainMap(c.Ctx, "responses")
	case "connector_response", "connector_responses":
		current = domainMap(domainMap(c.Ctx, "responses"), "connector")
	case "tool_response", "tool_responses":
		current = domainMap(domainMap(c.Ctx, "responses"), "tool")
	case "vars", "variables", "runtime":
		current = domainMap(c.Ctx, "variables")
	case "last":
		current = c.Last
	default:
		return nil, false
	}

	return readParts(current, parts[1:])
}

func (c *RunContext) Write(path string, value any) error {
	path = normalizePath(path)
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")

	var root map[string]any
	writePath := parts[1:]
	switch strings.ToLower(parts[0]) {
	case "input", "flow_input":
		return fmt.Errorf("flow_input is read-only")
	case "responses", "response", "connector_response", "connector_responses", "tool_response", "tool_responses":
		return fmt.Errorf("responses are read-only")
	case "flow_output", "output":
		if len(parts) < 2 {
			return fmt.Errorf("context write path must include a field: %s", path)
		}
		root = ensureDomainMap(&c.Ctx, "flow_output")
	case "vars", "variables", "runtime":
		if len(parts) < 2 {
			return fmt.Errorf("context write path must include a field: %s", path)
		}
		root = ensureDomainMap(&c.Ctx, "variables")
	case "ctx", "context":
		if len(parts) < 2 {
			return fmt.Errorf("context write path must include a field: %s", path)
		}
		switch strings.ToLower(parts[1]) {
		case "flow_input", "input":
			return fmt.Errorf("flow_input is read-only")
		case "responses", "response", "connector_response", "connector_responses", "tool_response", "tool_responses":
			return fmt.Errorf("responses are read-only")
		case "flow_output", "output":
			root = ensureDomainMap(&c.Ctx, "flow_output")
			parts = parts[1:]
			writePath = parts[1:]
		case "vars", "variables", "runtime":
			root = ensureDomainMap(&c.Ctx, "variables")
			parts = parts[1:]
			writePath = parts[1:]
		default:
			// Legacy ctx writes remain supported so older saved workflows keep
			// running. New UI writes to flow_output or variables explicitly.
			if c.Ctx == nil {
				c.Ctx = map[string]any{}
			}
			root = c.Ctx
		}
	default:
		// Bare write paths are treated as workflow ctx fields so UI configs can
		// use "feishu.document_id" instead of "$ctx.feishu.document_id".
		if c.Ctx == nil {
			c.Ctx = map[string]any{}
		}
		root = c.Ctx
		writePath = parts
	}

	writeParts(root, writePath, value)
	return nil
}

func (c *RunContext) writeSystem(path string, value any) error {
	path = normalizePath(path)
	parts := strings.Split(path, ".")
	if len(parts) < 3 || strings.ToLower(parts[0]) != "responses" {
		return c.Write(path, value)
	}
	root := ensureDomainMap(&c.Ctx, "responses")
	writeParts(root, parts[1:], value)
	return nil
}

func normalizeInitialContext(input map[string]any, initialContext map[string]any) map[string]any {
	ctx := cloneMap(initialContext)
	ctx["flow_input"] = cloneMap(input)
	if _, ok := ctx["flow_output"].(map[string]any); !ok {
		ctx["flow_output"] = map[string]any{}
	}
	responses, ok := ctx["responses"].(map[string]any)
	if !ok {
		responses = map[string]any{}
		ctx["responses"] = responses
	}
	if _, ok := responses["connector"].(map[string]any); !ok {
		responses["connector"] = map[string]any{}
	}
	if _, ok := responses["tool"].(map[string]any); !ok {
		responses["tool"] = map[string]any{}
	}
	if _, ok := ctx["variables"].(map[string]any); !ok {
		ctx["variables"] = map[string]any{}
	}
	return ctx
}

func domainMap(root map[string]any, key string) map[string]any {
	value, _ := root[key].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func ensureDomainMap(root *map[string]any, key string) map[string]any {
	if *root == nil {
		*root = map[string]any{}
	}
	value, ok := (*root)[key].(map[string]any)
	if !ok {
		value = map[string]any{}
		(*root)[key] = value
	}
	return value
}

func (c RunContext) Output() map[string]any {
	output := domainMap(c.Ctx, "flow_output")
	if len(output) > 0 {
		return cloneMap(output)
	}
	return cloneMap(c.Ctx)
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	return path
}

func readParts(current any, parts []string) (any, bool) {
	for _, part := range parts {
		if part == "" {
			continue
		}
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

func writeParts(root map[string]any, parts []string, value any) {
	current := root
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = cloneValue(value)
	}
	return result
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = cloneValue(item)
		}
		return result
	default:
		return value
	}
}
