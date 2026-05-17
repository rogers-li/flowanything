package expression

import (
	"fmt"
	"strconv"
	"strings"
)

func ReadPath(value any, path string) (any, bool) {
	parts := SplitPath(path)
	if len(parts) == 0 {
		return value, true
	}
	return readParts(value, parts)
}

func WritePath(target map[string]any, path string, value any) error {
	parts := SplitPath(path)
	if len(parts) == 0 {
		return fmt.Errorf("%w: write path must include at least one field", ErrInvalidPath)
	}
	writeParts(target, parts, value)
	return nil
}

func SplitPath(path string) []string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

func readParts(value any, parts []string) (any, bool) {
	current := value
	for _, part := range parts {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[part]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func writeParts(target map[string]any, parts []string, value any) {
	current := target
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
