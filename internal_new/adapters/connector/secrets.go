package connectoradapter

import (
	"os"
	"strings"
)

// SecretResolver keeps credential lookup outside core/connector.
type SecretResolver interface {
	ResolveSecret(ref string) (string, bool)
}

type EnvSecretResolver struct{}

func (EnvSecretResolver) ResolveSecret(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	for _, key := range envCandidates(ref) {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value, true
		}
	}
	return "", false
}

func envCandidates(ref string) []string {
	if strings.HasPrefix(ref, "env:") {
		key := strings.TrimSpace(strings.TrimPrefix(ref, "env:"))
		if key == "" {
			return nil
		}
		return []string{key}
	}
	if strings.HasPrefix(ref, "$") {
		key := strings.TrimSpace(strings.TrimPrefix(ref, "$"))
		if key == "" {
			return nil
		}
		return []string{key}
	}
	normalized := normalizeSecretRef(ref)
	if normalized == ref {
		return []string{ref}
	}
	return []string{ref, normalized}
}

func normalizeSecretRef(ref string) string {
	var builder strings.Builder
	lastUnderscore := false
	for _, value := range ref {
		switch {
		case value >= 'a' && value <= 'z':
			builder.WriteRune(value - 'a' + 'A')
			lastUnderscore = false
		case value >= 'A' && value <= 'Z':
			builder.WriteRune(value)
			lastUnderscore = false
		case value >= '0' && value <= '9':
			builder.WriteRune(value)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(builder.String(), "_")
}
