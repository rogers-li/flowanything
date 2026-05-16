package application

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
)

const maxModelFunctionNameLength = 64

func modelFunctionNameMapForTools(specs []tool.Spec) map[string]string {
	return modelFunctionNameMap(len(specs), func(index int) (key string, displayName string, fallback string) {
		spec := specs[index]
		return spec.ID.String(), spec.Name, firstNonEmpty(spec.ID.String(), spec.Name, "tool")
	})
}

func modelFunctionNameMapForSkills(specs []skill.Spec) map[string]string {
	return modelFunctionNameMap(len(specs), func(index int) (key string, displayName string, fallback string) {
		spec := specs[index]
		return spec.ID.String(), spec.Name, firstNonEmpty(spec.ID.String(), spec.Name, "skill")
	})
}

func modelFunctionNameMap(count int, item func(index int) (key string, displayName string, fallback string)) map[string]string {
	result := make(map[string]string, count)
	used := make(map[string]struct{}, count)
	for index := 0; index < count; index++ {
		key, displayName, fallback := item(index)
		if key == "" {
			key = fallback
		}
		name := safeModelFunctionName(displayName, fallback)
		if _, exists := used[name]; exists {
			name = uniqueModelFunctionName(name, fallback, used)
		}
		used[name] = struct{}{}
		result[key] = name
	}
	return result
}

func safeModelFunctionName(displayName string, fallback string) string {
	source := strings.TrimSpace(displayName)
	if source == "" {
		source = fallback
	}
	name := sanitizeModelFunctionName(source)
	if name == "" && strings.TrimSpace(fallback) != "" && strings.TrimSpace(fallback) != source {
		name = sanitizeModelFunctionName(fallback)
	}
	if name == "" {
		name = "tool"
	}
	return truncateModelFunctionName(name)
}

func sanitizeModelFunctionName(source string) string {
	var builder strings.Builder
	builder.Grow(len(source))
	previousUnderscore := false
	for _, r := range source {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			builder.WriteRune(r)
			previousUnderscore = false
		case r == '_':
			if !previousUnderscore {
				builder.WriteByte('_')
				previousUnderscore = true
			}
		default:
			if !previousUnderscore {
				builder.WriteByte('_')
				previousUnderscore = true
			}
		}
	}
	return strings.Trim(builder.String(), "_-")
}

func uniqueModelFunctionName(base string, fallback string, used map[string]struct{}) string {
	suffix := "_" + shortStableToken(fallback)
	name := truncateModelFunctionName(base)
	if len(name)+len(suffix) > maxModelFunctionNameLength {
		name = strings.TrimRight(name[:maxModelFunctionNameLength-len(suffix)], "_-")
	}
	if name == "" {
		name = "tool"
	}
	candidate := name + suffix
	for index := 2; ; index++ {
		if _, exists := used[candidate]; !exists {
			return candidate
		}
		nextSuffix := suffix + "_" + strconv.Itoa(index)
		if len(name)+len(nextSuffix) > maxModelFunctionNameLength {
			candidate = strings.TrimRight(name[:maxModelFunctionNameLength-len(nextSuffix)], "_-") + nextSuffix
		} else {
			candidate = name + nextSuffix
		}
	}
}

func truncateModelFunctionName(name string) string {
	if len(name) <= maxModelFunctionNameLength {
		return name
	}
	return strings.TrimRight(name[:maxModelFunctionNameLength], "_-")
}

func shortStableToken(value string) string {
	hash := sha1.Sum([]byte(value))
	return hex.EncodeToString(hash[:])[:8]
}
