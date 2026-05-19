package bootstrap

import (
	"context"
	"testing"

	coreconfig "flow-anything/core/config"
	configadapter "flow-anything/internal_new/adapters/config"
)

func TestNewRuntimeFromBundleFile(t *testing.T) {
	path := t.TempDir() + "/bundle.json"
	if err := configadapter.SaveBundleFile(path, testBundle()); err != nil {
		t.Fatalf("save bundle: %v", err)
	}
	runtime, err := NewRuntime(RuntimeConfig{BundlePath: path, MockContent: "booted"})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if runtime.Host == nil || runtime.Manager == nil || runtime.DebugSessions == nil || runtime.RunHistory == nil || runtime.ConfigService == nil || runtime.Handler == nil {
		t.Fatalf("runtime should include host and handler")
	}
}

func TestNewRuntimeFromBundleStore(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	bundle := testBundle()
	if err := store.SaveBundle(context.Background(), bundle); err != nil {
		t.Fatalf("save bundle: %v", err)
	}
	runtime, err := NewRuntime(RuntimeConfig{
		BundleStore: store,
		BundleID:    bundle.ID,
		MockContent: "booted from store",
	})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if runtime.Host == nil || runtime.Manager == nil || runtime.DebugSessions == nil || runtime.RunHistory == nil || runtime.ConfigService == nil || runtime.Handler == nil {
		t.Fatalf("runtime should include host and handler")
	}
}

func TestResolveDeepSeekModelDefaults(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-secret")
	config := RuntimeConfig{}
	bundle := testBundle()
	bundle.Resources.Models[0].Provider = "deepseek"
	bundle.Resources.Models[0].Model = "deepseek-v4-flash"

	provider := resolveModelProvider(config, bundle)
	if provider != "deepseek" {
		t.Fatalf("unexpected provider: %q", provider)
	}
	if baseURL := resolveModelBaseURL(provider, ""); baseURL != "https://api.deepseek.com" {
		t.Fatalf("unexpected base url: %q", baseURL)
	}
	if apiKey := resolveModelAPIKey(provider, ""); apiKey != "deepseek-secret" {
		t.Fatalf("unexpected api key: %q", apiKey)
	}
}

func TestExplicitModelConfigWinsOverProviderDefaults(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-secret")

	if baseURL := resolveModelBaseURL("deepseek", "https://example.test"); baseURL != "https://example.test" {
		t.Fatalf("unexpected base url: %q", baseURL)
	}
	if apiKey := resolveModelAPIKey("deepseek", "explicit-secret"); apiKey != "explicit-secret" {
		t.Fatalf("unexpected api key: %q", apiKey)
	}
}

func testBundle() coreconfig.BundleSpec {
	return coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		Kind:          coreconfig.BundleKind,
		ID:            "bundle_bootstrap_test",
		Name:          "Bootstrap Test Bundle",
		Version:       "v1",
		Metadata: map[string]any{
			coreconfig.BundleMetadataLifecycle: string(coreconfig.BundleLifecycleRelease),
		},
		Runtime: coreconfig.RuntimeTargetSpec{
			Targets: []coreconfig.RuntimeTarget{coreconfig.RuntimeTest},
		},
		Resources: coreconfig.ResourceCollection{
			Models: []coreconfig.ModelConfig{
				{
					ResourceMeta: coreconfig.ResourceMeta{ID: "model_mock", Name: "Mock Model"},
					Provider:     "mock",
					Model:        "mock-chat",
				},
			},
			Agents: []coreconfig.AgentConfig{
				{
					ResourceMeta: coreconfig.ResourceMeta{ID: "agent_help", Name: "Help Agent"},
					Prompt:       coreconfig.PromptConfig{System: "You are helpful."},
					Reasoning:    coreconfig.ReasoningConfig{Mode: "direct"},
					ModelRef:     coreconfig.ResourceRef{Kind: coreconfig.ResourceModel, ID: "model_mock"},
				},
			},
		},
	}
}
