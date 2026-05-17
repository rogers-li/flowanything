package expression

import "fmt"

func ResolveValue(ctx Context, nodeOutput map[string]any, source ValueSource) (any, error) {
	switch source.Type {
	case SourceConst:
		return source.Value, nil
	case SourceContext:
		value, ok := ctx.Read(source.Path)
		if !ok {
			return nil, fmt.Errorf("%w: context %q", ErrPathNotFound, source.Path)
		}
		return value, nil
	case SourceNodeOutput:
		if source.Path == "" || source.Path == "$" {
			return nodeOutput, nil
		}
		value, ok := ReadPath(nodeOutput, source.Path)
		if !ok {
			return nil, fmt.Errorf("%w: node_output %q", ErrPathNotFound, source.Path)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownSourceType, source.Type)
	}
}

func BuildObject(ctx Context, bindings []FieldBinding) (map[string]any, error) {
	out := map[string]any{}
	for _, binding := range bindings {
		if !binding.Enabled {
			continue
		}
		if binding.Field == "" {
			return nil, fmt.Errorf("%w: binding field is required", ErrInvalidPath)
		}
		value, err := ResolveValue(ctx, nil, binding.Source)
		if err != nil {
			return nil, fmt.Errorf("resolve field %q: %w", binding.Field, err)
		}
		if err := WritePath(out, binding.Field, value); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func ApplyContextWrites(ctx Context, nodeOutput map[string]any, writes []ContextWrite) error {
	for _, write := range writes {
		if !write.Enabled {
			continue
		}
		if write.Target == "" {
			return fmt.Errorf("%w: context write target is required", ErrInvalidPath)
		}
		value, err := ResolveValue(ctx, nodeOutput, write.Source)
		if err != nil {
			return fmt.Errorf("resolve write %q: %w", write.Target, err)
		}
		if err := ctx.Write(write.Target, value); err != nil {
			return err
		}
	}
	return nil
}
