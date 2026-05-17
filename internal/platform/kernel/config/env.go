package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the environment value for key, or fallback when the key is not set.
func String(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

// Duration returns a parsed duration from the environment.
//
// The value accepts Go duration strings such as "30s" or "1500ms". For
// operator convenience, a plain integer is interpreted as milliseconds.
func Duration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if parsed, err := time.ParseDuration(value); err == nil {
		return parsed
	}
	if millis, err := strconv.Atoi(value); err == nil && millis > 0 {
		return time.Duration(millis) * time.Millisecond
	}
	return fallback
}

// Int returns a positive integer from the environment, or fallback when unset or invalid.
func Int(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// Bool returns a boolean from the environment, or fallback when unset or invalid.
func Bool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
