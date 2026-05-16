package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

func decodeNodeConfig[T any](config map[string]any) (T, error) {
	var decoded T
	if config == nil {
		return decoded, fmt.Errorf("node config is required")
	}
	data, err := json.Marshal(config)
	if err != nil {
		return decoded, err
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return decoded, err
	}
	return decoded, nil
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		if nested, ok := value.(map[string]any); ok {
			cloned[key] = cloneMap(nested)
			continue
		}
		cloned[key] = value
	}
	return cloned
}

func stringSliceArg(args map[string]any, key string) ([]string, error) {
	value, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("arg %q is required", key)
	}
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, fmt.Sprint(item))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("arg %q must be string array", key)
	}
}

func readNested(data map[string]any, path string) any {
	current := any(data)
	for _, part := range splitPath(path) {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = asMap[part]
	}
	return current
}

func writeNested(data map[string]any, path string, value any) {
	parts := splitPath(path)
	current := data
	for i, part := range parts {
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

func deleteNested(data map[string]any, path string) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return
	}
	current := data
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return
		}
		current = next
	}
	delete(current, parts[len(parts)-1])
}

func splitPath(path string) []string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}
