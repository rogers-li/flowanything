package flowengine

import (
	"fmt"
	"strings"

	"flow-anything/core/expression"
)

// DataContext is the runtime context shared by all nodes.
//
// FlowInput is read-only by convention. FlowOutput and Variables are writable
// through ContextWrite. NodeContext stores extension data owned by node types,
// for example connector responses or agent intermediate artifacts.
type DataContext struct {
	FlowInput   map[string]any
	FlowOutput  map[string]any
	Variables   map[string]any
	NodeContext map[string]any
}

// NewDataContext creates a context with the standard top-level domains.
func NewDataContext(flowInput map[string]any) *DataContext {
	return &DataContext{
		FlowInput:   cloneMap(flowInput),
		FlowOutput:  map[string]any{},
		Variables:   map[string]any{},
		NodeContext: map[string]any{},
	}
}

// Read returns a value from a path such as $.flow_input.query or
// $.variables.normalized_query.
func (c *DataContext) Read(path string) (any, bool) {
	root, parts := splitPath(path)
	var current any
	switch root {
	case "flow_input", "input":
		current = c.FlowInput
	case "flow_output", "output":
		current = c.FlowOutput
	case "variables", "vars":
		current = c.Variables
	case "node_context", "node_contexts", "nodes":
		current = c.NodeContext
	default:
		return nil, false
	}
	return expression.ReadPath(current, strings.Join(parts, "."))
}

// Write stores value into a writable context path. FlowInput is intentionally
// rejected so node implementations cannot mutate caller input.
func (c *DataContext) Write(path string, value any) error {
	root, parts := splitPath(path)
	if len(parts) == 0 {
		return fmt.Errorf("context write path %q must include a field name", path)
	}
	switch root {
	case "flow_input", "input":
		return fmt.Errorf("flow input is read-only")
	case "flow_output", "output":
		return expression.WritePath(c.FlowOutput, strings.Join(parts, "."), value)
	case "variables", "vars":
		return expression.WritePath(c.Variables, strings.Join(parts, "."), value)
	case "node_context", "node_contexts", "nodes":
		return expression.WritePath(c.NodeContext, strings.Join(parts, "."), value)
	default:
		return fmt.Errorf("unknown context root %q", root)
	}
}

// ReadNodeContext reads data from a namespace owned by a node type or adapter.
// For example, connector nodes can read namespace "connector" with path
// "responses.search".
func (c *DataContext) ReadNodeContext(namespace, path string) (any, bool) {
	if namespace == "" {
		return nil, false
	}
	namespaceData, ok := c.NodeContext[namespace]
	if !ok {
		return nil, false
	}
	return expression.ReadPath(namespaceData, path)
}

// WriteNodeContext writes data to a namespace owned by a node type or adapter.
// The engine does not interpret the namespace; upper-layer node implementations
// define its schema and semantics.
func (c *DataContext) WriteNodeContext(namespace, path string, value any) error {
	if namespace == "" {
		return fmt.Errorf("node context namespace is required")
	}
	if path == "" {
		return fmt.Errorf("node context path is required")
	}
	namespaceData, ok := c.NodeContext[namespace].(map[string]any)
	if !ok {
		namespaceData = map[string]any{}
		c.NodeContext[namespace] = namespaceData
	}
	return expression.WritePath(namespaceData, path, value)
}

func splitPath(path string) (string, []string) {
	parts := expression.SplitPath(path)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
