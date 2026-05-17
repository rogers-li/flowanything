package workflowruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	apperrors "flow-anything/internal/platform/kernel/errors"
)

type transformFunction func(context.Context, map[string]any) (map[string]any, error)

var transformFunctions = map[string]transformFunction{
	"json.extract_path":  transformExtractPath,
	"json.pick_fields":   transformPickFields,
	"json.remove_fields": transformRemoveFields,
	"json.rename_fields": transformRenameFields,
	"json.set_value":     transformSetValue,
	"array.chunk":        transformArrayChunk,
	"array.filter_empty": transformFilterEmpty,
	"string.template":    transformStringTemplate,
	"type.convert":       transformTypeConvert,
}

func ExecuteTransformFunction(ctx context.Context, functionID string, input map[string]any) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fn, ok := transformFunctions[strings.TrimSpace(functionID)]
	if !ok {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "unsupported transform function")
	}
	return fn(ctx, input)
}

func transformRemoveFields(ctx context.Context, input map[string]any) (map[string]any, error) {
	fields, err := stringSliceInput(input, "fields")
	if err != nil {
		return nil, err
	}
	fieldSet := map[string]bool{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			fieldSet[field] = true
		}
	}
	if len(fieldSet) == 0 {
		return nil, invalidTransformInput("fields must contain at least one field name")
	}
	recursive := boolInput(input, "recursive", true)
	result, count, paths := removeFields(cloneTransformValue(input["value"]), fieldSet, recursive, "")
	return map[string]any{
		"result":        result,
		"removed_count": count,
		"removed_paths": paths,
	}, nil
}

func transformPickFields(ctx context.Context, input map[string]any) (map[string]any, error) {
	value := input["value"]
	fields, err := stringSliceInput(input, "fields")
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	for _, field := range fields {
		path := strings.TrimSpace(field)
		if path == "" {
			continue
		}
		if got, ok := readTransformPath(value, path); ok {
			writeTransformPath(result, path, cloneTransformValue(got), true)
		}
	}
	return map[string]any{"result": result, "picked_count": len(result)}, nil
}

func transformRenameFields(ctx context.Context, input map[string]any) (map[string]any, error) {
	renameMap, err := stringMapInput(input, "rename_map")
	if err != nil {
		return nil, err
	}
	if len(renameMap) == 0 {
		return nil, invalidTransformInput("rename_map must contain at least one field mapping")
	}
	recursive := boolInput(input, "recursive", false)
	result, count := renameFields(cloneTransformValue(input["value"]), renameMap, recursive)
	return map[string]any{
		"result":        result,
		"renamed_count": count,
	}, nil
}

func transformSetValue(ctx context.Context, input map[string]any) (map[string]any, error) {
	path := strings.TrimSpace(stringInput(input, "path", ""))
	if path == "" {
		return nil, invalidTransformInput("path is required")
	}
	result := cloneTransformValue(input["value"])
	if result == nil {
		result = map[string]any{}
	}
	root, ok := result.(map[string]any)
	if !ok {
		return nil, invalidTransformInput("value must be an object for json.set_value")
	}
	created := writeTransformPath(root, path, cloneTransformValue(input["new_value"]), boolInput(input, "create_missing", true))
	return map[string]any{
		"result":  root,
		"updated": created,
	}, nil
}

func transformExtractPath(ctx context.Context, input map[string]any) (map[string]any, error) {
	path := strings.TrimSpace(stringInput(input, "path", ""))
	if path == "" {
		return nil, invalidTransformInput("path is required")
	}
	value, ok := readTransformPath(input["value"], path)
	return map[string]any{
		"result": cloneTransformValue(value),
		"exists": ok,
	}, nil
}

func transformArrayChunk(ctx context.Context, input map[string]any) (map[string]any, error) {
	items, ok := toAnySlice(input["value"])
	if !ok {
		return nil, invalidTransformInput("value must be an array")
	}
	size := intInput(input, "size", 1000)
	if size <= 0 {
		return nil, invalidTransformInput("size must be greater than 0")
	}
	chunks := make([]any, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunk := make([]any, end-start)
		copy(chunk, items[start:end])
		chunks = append(chunks, chunk)
	}
	return map[string]any{
		"chunks": chunks,
		"count":  len(chunks),
	}, nil
}

func transformFilterEmpty(ctx context.Context, input map[string]any) (map[string]any, error) {
	result, removed := filterEmptyValues(cloneTransformValue(input["value"]), boolInput(input, "recursive", true))
	return map[string]any{
		"result":        result,
		"removed_count": removed,
	}, nil
}

func transformStringTemplate(ctx context.Context, input map[string]any) (map[string]any, error) {
	template := stringInput(input, "template", "")
	variables, ok := input["variables"].(map[string]any)
	if !ok || variables == nil {
		variables = input
	}
	result := templatePlaceholder.ReplaceAllStringFunc(template, func(match string) string {
		key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(match, "{{"), "}}"))
		value, ok := readTransformPath(variables, key)
		if !ok || value == nil {
			return ""
		}
		return fmt.Sprint(value)
	})
	return map[string]any{"result": result}, nil
}

func transformTypeConvert(ctx context.Context, input map[string]any) (map[string]any, error) {
	targetType := strings.ToLower(strings.TrimSpace(stringInput(input, "target_type", "string")))
	result, err := convertTransformValue(input["value"], targetType)
	if err != nil {
		return nil, err
	}
	return map[string]any{"result": result}, nil
}

var templatePlaceholder = regexp.MustCompile(`\{\{\s*[^{}]+\s*\}\}`)

func removeFields(value any, fields map[string]bool, recursive bool, basePath string) (any, int, []string) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		count := 0
		paths := []string{}
		for key, child := range typed {
			path := joinTransformPath(basePath, key)
			if fields[key] {
				count++
				paths = append(paths, path)
				continue
			}
			if recursive {
				nextChild, nextCount, nextPaths := removeFields(child, fields, recursive, path)
				result[key] = nextChild
				count += nextCount
				paths = append(paths, nextPaths...)
			} else {
				result[key] = child
			}
		}
		return result, count, paths
	case []any:
		if !recursive {
			return typed, 0, nil
		}
		result := make([]any, len(typed))
		count := 0
		paths := []string{}
		for index, child := range typed {
			nextChild, nextCount, nextPaths := removeFields(child, fields, recursive, joinTransformPath(basePath, strconv.Itoa(index)))
			result[index] = nextChild
			count += nextCount
			paths = append(paths, nextPaths...)
		}
		return result, count, paths
	default:
		return typed, 0, nil
	}
}

func renameFields(value any, renameMap map[string]string, recursive bool) (any, int) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		count := 0
		for key, child := range typed {
			nextKey := key
			if renamed := strings.TrimSpace(renameMap[key]); renamed != "" {
				nextKey = renamed
				count++
			}
			if recursive {
				nextChild, nextCount := renameFields(child, renameMap, recursive)
				result[nextKey] = nextChild
				count += nextCount
			} else {
				result[nextKey] = child
			}
		}
		return result, count
	case []any:
		if !recursive {
			return typed, 0
		}
		result := make([]any, len(typed))
		count := 0
		for index, child := range typed {
			nextChild, nextCount := renameFields(child, renameMap, recursive)
			result[index] = nextChild
			count += nextCount
		}
		return result, count
	default:
		return typed, 0
	}
}

func filterEmptyValues(value any, recursive bool) (any, int) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		removed := 0
		for key, child := range typed {
			next := child
			nextRemoved := 0
			if recursive {
				next, nextRemoved = filterEmptyValues(child, recursive)
			}
			removed += nextRemoved
			if isEmptyTransformValue(next) {
				removed++
				continue
			}
			result[key] = next
		}
		return result, removed
	case []any:
		result := make([]any, 0, len(typed))
		removed := 0
		for _, child := range typed {
			next := child
			nextRemoved := 0
			if recursive {
				next, nextRemoved = filterEmptyValues(child, recursive)
			}
			removed += nextRemoved
			if isEmptyTransformValue(next) {
				removed++
				continue
			}
			result = append(result, next)
		}
		return result, removed
	default:
		return typed, 0
	}
}

func isEmptyTransformValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func readTransformPath(root any, path string) (any, bool) {
	if strings.TrimSpace(path) == "" {
		return root, true
	}
	current := root
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
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

func writeTransformPath(root map[string]any, path string, value any, createMissing bool) bool {
	parts := strings.Split(path, ".")
	current := root
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if index == len(parts)-1 {
			current[part] = value
			return true
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			if !createMissing {
				return false
			}
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	return false
}

func convertTransformValue(value any, targetType string) (any, error) {
	switch targetType {
	case "string":
		if value == nil {
			return "", nil
		}
		return fmt.Sprint(value), nil
	case "number", "float":
		switch typed := value.(type) {
		case float64:
			return typed, nil
		case int:
			return float64(typed), nil
		case json.Number:
			return typed.Float64()
		case string:
			return strconv.ParseFloat(strings.TrimSpace(typed), 64)
		default:
			return nil, invalidTransformInput("value cannot be converted to number")
		}
	case "integer", "int":
		switch typed := value.(type) {
		case int:
			return typed, nil
		case float64:
			return int(typed), nil
		case json.Number:
			got, err := typed.Int64()
			return int(got), err
		case string:
			got, err := strconv.Atoi(strings.TrimSpace(typed))
			return got, err
		default:
			return nil, invalidTransformInput("value cannot be converted to integer")
		}
	case "boolean", "bool":
		switch typed := value.(type) {
		case bool:
			return typed, nil
		case string:
			return strconv.ParseBool(strings.TrimSpace(typed))
		default:
			return nil, invalidTransformInput("value cannot be converted to boolean")
		}
	case "object", "array":
		if text, ok := value.(string); ok {
			var parsed any
			if err := json.Unmarshal([]byte(text), &parsed); err != nil {
				return nil, err
			}
			if targetType == "object" {
				if _, ok := parsed.(map[string]any); !ok {
					return nil, invalidTransformInput("parsed value is not an object")
				}
			}
			if targetType == "array" {
				if _, ok := parsed.([]any); !ok {
					return nil, invalidTransformInput("parsed value is not an array")
				}
			}
			return parsed, nil
		}
		return cloneTransformValue(value), nil
	default:
		return nil, invalidTransformInput("unsupported target_type")
	}
}

func stringSliceInput(input map[string]any, key string) ([]string, error) {
	value, ok := input[key]
	if !ok {
		return nil, invalidTransformInput(key + " is required")
	}
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, strings.TrimSpace(fmt.Sprint(item)))
		}
		return result, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
		if strings.HasPrefix(strings.TrimSpace(typed), "[") {
			var result []string
			if err := json.Unmarshal([]byte(typed), &result); err == nil {
				return result, nil
			}
		}
		parts := strings.Split(typed, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			result = append(result, strings.TrimSpace(part))
		}
		return result, nil
	default:
		return nil, invalidTransformInput(key + " must be an array or comma-separated string")
	}
}

func stringMapInput(input map[string]any, key string) (map[string]string, error) {
	value, ok := input[key]
	if !ok {
		return nil, invalidTransformInput(key + " is required")
	}
	switch typed := value.(type) {
	case map[string]string:
		return typed, nil
	case map[string]any:
		result := make(map[string]string, len(typed))
		for k, v := range typed {
			result[k] = fmt.Sprint(v)
		}
		return result, nil
	case string:
		var raw map[string]string
		if err := json.Unmarshal([]byte(typed), &raw); err != nil {
			return nil, err
		}
		return raw, nil
	default:
		return nil, invalidTransformInput(key + " must be an object")
	}
}

func stringInput(input map[string]any, key string, fallback string) string {
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}
	return fmt.Sprint(value)
}

func boolInput(input map[string]any, key string, fallback bool) bool {
	value, ok := input[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		got, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err == nil {
			return got
		}
	}
	return fallback
}

func intInput(input map[string]any, key string, fallback int) int {
	value, ok := input[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		got, err := typed.Int64()
		if err == nil {
			return int(got)
		}
	case string:
		got, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return got
		}
	}
	return fallback
}

func toAnySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []map[string]any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func cloneTransformValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = cloneTransformValue(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = cloneTransformValue(item)
		}
		return result
	default:
		return typed
	}
}

func joinTransformPath(base string, part string) string {
	if base == "" {
		return part
	}
	return base + "." + part
}

func invalidTransformInput(message string) error {
	return apperrors.New(apperrors.CodeInvalidArgument, message)
}
