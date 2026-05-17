package trace

import "strings"

type Redactor interface {
	RedactMap(input map[string]any) map[string]any
}

type FieldRedactor struct {
	SensitiveKeys []string
	Replacement   string
}

func NewDefaultRedactor() FieldRedactor {
	return FieldRedactor{
		SensitiveKeys: []string{"api_key", "apikey", "authorization", "token", "secret", "password", "refresh_token"},
		Replacement:   "[redacted]",
	}
}

func (r FieldRedactor) RedactMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	replacement := r.Replacement
	if replacement == "" {
		replacement = "[redacted]"
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		if r.isSensitive(key) {
			out[key] = replacement
			continue
		}
		out[key] = r.redactValue(value)
	}
	return out
}

func (r FieldRedactor) redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return r.RedactMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = r.redactValue(item)
		}
		return out
	default:
		return value
	}
}

func (r FieldRedactor) isSensitive(key string) bool {
	lower := strings.ToLower(key)
	for _, candidate := range r.SensitiveKeys {
		if strings.Contains(lower, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}
