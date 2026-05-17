package flowengine

import "flow-anything/core/expression"

// BuildNodeInput applies node input mappings against the current context.
func BuildNodeInput(data *DataContext, mappings []FieldBinding) (map[string]any, error) {
	if len(mappings) == 0 {
		return cloneMap(data.FlowInput), nil
	}
	return expression.BuildObject(data, mappings)
}

// ApplyContextWrites writes selected node outputs back to the shared context.
func ApplyContextWrites(data *DataContext, nodeOutput map[string]any, writes []ContextWrite) error {
	return expression.ApplyContextWrites(data, nodeOutput, writes)
}

// ResolveValue resolves one mapping source.
func ResolveValue(data *DataContext, nodeOutput map[string]any, source ValueSource) (any, error) {
	return expression.ResolveValue(data, nodeOutput, source)
}
