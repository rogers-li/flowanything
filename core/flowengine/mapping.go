package flowengine

import (
	"fmt"
	"strings"
)

// BuildNodeInput applies node input mappings against the current context.
func BuildNodeInput(data *DataContext, mappings []FieldBinding) (map[string]any, error) {
	input := map[string]any{}
	for _, binding := range mappings {
		if !binding.Enabled {
			continue
		}
		if binding.Field == "" {
			return nil, fmt.Errorf("input mapping field is required")
		}
		value, err := ResolveValue(data, nil, binding.Source)
		if err != nil {
			return nil, fmt.Errorf("resolve input field %q: %w", binding.Field, err)
		}
		writeParts(input, splitField(binding.Field), value)
	}
	return input, nil
}

// ApplyContextWrites writes selected node outputs back to the shared context.
func ApplyContextWrites(data *DataContext, nodeOutput map[string]any, writes []ContextWrite) error {
	for _, write := range writes {
		if !write.Enabled {
			continue
		}
		if write.Target == "" {
			return fmt.Errorf("context write target is required")
		}
		value, err := ResolveValue(data, nodeOutput, write.Source)
		if err != nil {
			return fmt.Errorf("resolve context write %q: %w", write.Target, err)
		}
		if err := data.Write(write.Target, value); err != nil {
			return err
		}
	}
	return nil
}

// ResolveValue resolves one mapping source.
func ResolveValue(data *DataContext, nodeOutput map[string]any, source ValueSource) (any, error) {
	switch source.Type {
	case SourceConst:
		return source.Value, nil
	case SourceContext:
		value, ok := data.Read(source.Path)
		if !ok {
			return nil, fmt.Errorf("context path %q not found", source.Path)
		}
		return value, nil
	case SourceNodeOutput:
		path := source.Path
		if path == "" || path == "$" {
			return nodeOutput, nil
		}
		parts := splitField(path)
		value, ok := readParts(nodeOutput, parts)
		if !ok {
			return nil, fmt.Errorf("node output path %q not found", source.Path)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown value source type %q", source.Type)
	}
}

func splitField(field string) []string {
	field = strings.TrimSpace(field)
	field = strings.TrimPrefix(field, "$.")
	field = strings.TrimPrefix(field, "$")
	field = strings.TrimPrefix(field, ".")
	if field == "" {
		return nil
	}
	return strings.Split(field, ".")
}
