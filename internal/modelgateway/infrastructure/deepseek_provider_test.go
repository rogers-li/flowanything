package infrastructure

import (
	"testing"
	"time"
)

func TestNewDeepSeekProviderAppliesDefaults(t *testing.T) {
	t.Parallel()

	provider, err := NewDeepSeekProvider(DeepSeekConfig{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}
	if provider.delegate.config.BaseURL != DefaultDeepSeekBaseURL {
		t.Fatalf("expected default base url, got %q", provider.delegate.config.BaseURL)
	}
	if provider.delegate.config.DefaultModel != DefaultDeepSeekModel {
		t.Fatalf("expected default model, got %q", provider.delegate.config.DefaultModel)
	}
}

func TestNewDeepSeekProviderMapsThinkingOptions(t *testing.T) {
	t.Parallel()

	provider, err := NewDeepSeekProvider(DeepSeekConfig{
		APIKey:          "test-key",
		DefaultModel:    "deepseek-v4-pro",
		ThinkingType:    "enabled",
		ReasoningEffort: "high",
		Timeout:         12 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}
	thinking, _ := provider.delegate.config.ExtraBody["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Fatalf("expected thinking enabled, got %#v", provider.delegate.config.ExtraBody["thinking"])
	}
	if provider.delegate.config.ExtraBody["reasoning_effort"] != "high" {
		t.Fatalf("expected reasoning effort high, got %#v", provider.delegate.config.ExtraBody["reasoning_effort"])
	}
	if provider.delegate.config.Timeout != 12*time.Second {
		t.Fatalf("unexpected timeout %s", provider.delegate.config.Timeout)
	}
}

func TestNewDeepSeekProviderRequiresAPIKey(t *testing.T) {
	t.Parallel()

	if _, err := NewDeepSeekProvider(DeepSeekConfig{}); err == nil {
		t.Fatal("expected missing api key error")
	}
}
