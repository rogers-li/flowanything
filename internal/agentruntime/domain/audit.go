package domain

import (
	"fmt"
	"reflect"
	"time"

	"flow-anything/internal/platform/contracts/tool"
)

// NewExecutionRecord creates the audit record captured at the beginning of a
// tool execution attempt. It intentionally stores an argument summary rather
// than raw arguments to avoid leaking sensitive data into audit logs by default.
func NewExecutionRecord(execution Execution, spec tool.Spec) tool.ExecutionRecord {
	call := execution.Call
	return tool.ExecutionRecord{
		CallID:               call.ID,
		TenantID:             call.TenantID,
		ToolID:               call.ToolID,
		ToolName:             valueOrFallback(spec.Name, call.Name),
		Implementation:       firstImplementation(spec.Implementation, call.Implementation),
		RiskLevel:            spec.RiskLevel,
		RequiresConfirmation: spec.RequiresExecutionConfirmation(),
		Confirmed:            call.Confirmed,
		SpecVersion:          spec.Version,
		TraceID:              call.TraceID,
		ArgsSummary:          SummarizeArgs(call.Args),
		Status:               tool.ExecutionStatusStarted,
		StartedAt:            execution.StartedAt,
	}
}

// CompleteExecutionRecord finalizes an audit record with normalized result
// status, timing, and failure information.
func CompleteExecutionRecord(record tool.ExecutionRecord, result tool.Result) tool.ExecutionRecord {
	if result.StartedAt.IsZero() {
		result.StartedAt = record.StartedAt
	}
	if result.FinishedAt.IsZero() {
		result.FinishedAt = time.Now().UTC()
	}
	if result.CallID.Empty() {
		result.CallID = record.CallID
	}
	if result.ToolID.Empty() {
		result.ToolID = record.ToolID
	}

	record.Result = &result
	record.ErrorCode = result.ErrorCode
	record.ErrorReason = result.ErrorReason
	record.FinishedAt = result.FinishedAt
	record.DurationMillis = result.FinishedAt.Sub(record.StartedAt).Milliseconds()
	if result.Success {
		record.Status = tool.ExecutionStatusSucceeded
	} else {
		record.Status = tool.ExecutionStatusFailed
	}

	return record
}

func SummarizeArgs(args map[string]any) map[string]any {
	if len(args) == 0 {
		return nil
	}

	summary := make(map[string]any, len(args))
	for key, value := range args {
		summary[key] = summarizeValue(value)
	}
	return summary
}

func summarizeValue(value any) map[string]any {
	result := map[string]any{
		"type": valueType(value),
	}
	if value == nil {
		result["empty"] = true
		return result
	}

	switch typed := value.(type) {
	case string:
		result["length"] = len([]rune(typed))
		result["empty"] = typed == ""
	case map[string]any:
		result["field_count"] = len(typed)
	case []any:
		result["length"] = len(typed)
	default:
		kind := reflect.ValueOf(value).Kind()
		if kind == reflect.Slice || kind == reflect.Array {
			result["length"] = reflect.ValueOf(value).Len()
		}
	}
	return result
}

func valueType(value any) string {
	if value == nil {
		return "null"
	}
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		kind := reflect.ValueOf(value).Kind()
		switch kind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			return "number"
		case reflect.Slice, reflect.Array:
			return "array"
		default:
			return fmt.Sprintf("%T", value)
		}
	}
}

func valueOrFallback(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func firstImplementation(value tool.ImplementationType, fallback tool.ImplementationType) tool.ImplementationType {
	if value != "" {
		return value
	}
	return fallback
}
