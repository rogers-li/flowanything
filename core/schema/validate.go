package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// ValidateDefinition checks that a schema is structurally valid for both UI
// editing and runtime validation.
func ValidateDefinition(fields Schema) error {
	var errs ValidationErrors
	validateFieldsDefinition(&errs, "$", fields)
	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidateValue validates an object against the given schema. Extra fields are
// allowed so config authors can evolve contracts without breaking older
// runtimes.
func ValidateValue(fields Schema, value map[string]any) error {
	var errs ValidationErrors
	validateObjectValue(&errs, "$", fields, value)
	if errs.HasErrors() {
		return errs
	}
	return nil
}

func validateFieldsDefinition(errs *ValidationErrors, path string, fields []Field) {
	seen := map[string]bool{}
	for i, field := range fields {
		fieldPath := path + "." + field.Name
		if field.Name == "" {
			errs.Add(fmt.Sprintf("%s[%d].name", path, i), "field name is required")
			continue
		}
		if seen[field.Name] {
			errs.Add(fieldPath, "duplicate field name")
		}
		seen[field.Name] = true
		if field.Type == "" {
			errs.Add(fieldPath+".type", "field type is required")
		}
		if field.Type == TypeObject || len(field.Children) > 0 {
			validateFieldsDefinition(errs, fieldPath, field.Children)
		}
	}
}

func validateObjectValue(errs *ValidationErrors, path string, fields []Field, value map[string]any) {
	for _, field := range fields {
		fieldPath := path + "." + field.Name
		raw, exists := value[field.Name]
		if !exists || raw == nil {
			if field.Required {
				errs.Add(fieldPath, "required field is missing")
			}
			continue
		}
		validateFieldValue(errs, fieldPath, field, raw)
	}
}

func validateFieldValue(errs *ValidationErrors, path string, field Field, value any) {
	switch normalizeType(field.Type) {
	case TypeAny:
		return
	case TypeString:
		if _, ok := value.(string); !ok {
			errs.Add(path, "expected string")
		}
	case TypeNumber:
		if !isNumber(value) {
			errs.Add(path, "expected number")
		}
	case TypeInteger:
		if !isInteger(value) {
			errs.Add(path, "expected integer")
		}
	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			errs.Add(path, "expected boolean")
		}
	case TypeObject:
		objectValue, ok := value.(map[string]any)
		if !ok {
			errs.Add(path, "expected object")
			return
		}
		validateObjectValue(errs, path, field.Children, objectValue)
	case TypeArray:
		arrayValue, ok := value.([]any)
		if !ok {
			errs.Add(path, "expected array")
			return
		}
		if len(field.Children) == 0 {
			return
		}
		for i, item := range arrayValue {
			objectItem, ok := item.(map[string]any)
			if !ok {
				errs.Add(fmt.Sprintf("%s[%d]", path, i), "expected object array item")
				continue
			}
			validateObjectValue(errs, fmt.Sprintf("%s[%d]", path, i), field.Children, objectItem)
		}
	default:
		errs.Add(path+".type", fmt.Sprintf("unsupported field type %q", field.Type))
	}
}

func normalizeType(fieldType FieldType) FieldType {
	if fieldType == "" {
		return TypeAny
	}
	return fieldType
}

func isNumber(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	case json.Number:
		return true
	default:
		return false
	}
}

func isInteger(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return math.Trunc(float64(typed)) == float64(typed)
	case float64:
		return math.Trunc(typed) == typed
	case json.Number:
		_, err := strconv.ParseInt(typed.String(), 10, 64)
		return err == nil
	default:
		return false
	}
}
