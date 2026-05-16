package domain

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	apperrors "flow-anything/internal/platform/kernel/errors"
)

// ValidateInputSchema validates tool arguments against the JSON-schema subset
// used by Tool input_schema.
//
// The first enterprise-grade Runtime guardrail is intentionally small and
// deterministic: required fields plus primitive type checks. More JSON Schema
// keywords can be added here without changing adapters or service contracts.
func ValidateInputSchema(args map[string]any, schema map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	if args == nil {
		args = map[string]any{}
	}

	requiredFields := stringSlice(schema["required"])
	for _, field := range requiredFields {
		value, ok := args[field]
		if !ok || value == nil {
			return apperrors.New(apperrors.CodeInvalidArgument, fmt.Sprintf("tool argument %q is required", field))
		}
	}

	properties, _ := schema["properties"].(map[string]any)
	for name, rawProperty := range properties {
		value, ok := args[name]
		if !ok || value == nil {
			continue
		}
		property, _ := rawProperty.(map[string]any)
		if err := validateValue(name, value, property); err != nil {
			return err
		}
	}

	return nil
}

func validateValue(name string, value any, property map[string]any) error {
	expectedType := strings.TrimSpace(fmt.Sprint(property["type"]))
	if expectedType != "" && !matchesType(value, expectedType) {
		return apperrors.New(
			apperrors.CodeInvalidArgument,
			fmt.Sprintf("tool argument %q must be %s", name, expectedType),
		)
	}

	if enumValues := property["enum"]; enumValues != nil && !matchesEnum(value, enumValues) {
		return apperrors.New(
			apperrors.CodeInvalidArgument,
			fmt.Sprintf("tool argument %q must be one of the allowed values", name),
		)
	}

	return nil
}

func matchesType(value any, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		return isInteger(value)
	case "number":
		_, ok := numericValue(value)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		return reflect.ValueOf(value).Kind() == reflect.Slice
	default:
		// Unknown schema types should not block execution; they can be handled by
		// stricter validators once the platform adopts a full JSON Schema engine.
		return true
	}
}

func isInteger(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return math.Trunc(typed) == typed
	case float32:
		return math.Trunc(float64(typed)) == float64(typed)
	default:
		return false
	}
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func matchesEnum(value any, enumValues any) bool {
	for _, item := range anySlice(enumValues) {
		if fmt.Sprint(item) == fmt.Sprint(value) {
			return true
		}
	}
	return false
}

func stringSlice(value any) []string {
	result := make([]string, 0)
	for _, item := range anySlice(value) {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func anySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}
