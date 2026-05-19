package app

import (
	"context"
	"testing"

	coreconfig "flow-anything/core/config"
)

func TestRuntimeManagerReloadsHostAtomically(t *testing.T) {
	ctx := context.Background()
	initialBundle := releaseTestBundle(baseTestBundle())
	initialHost, err := NewHost(initialBundle, fakeModelClient{content: "initial"})
	if err != nil {
		t.Fatalf("new initial host: %v", err)
	}
	factory := func(ctx context.Context, bundle coreconfig.BundleSpec) (*Host, error) {
		return NewHost(bundle, fakeModelClient{content: "answer from " + bundle.ID})
	}
	manager, err := NewRuntimeManager(initialHost, initialBundle, factory)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	nextBundle := baseTestBundle()
	nextBundle.ID = "bundle_next"
	nextBundle.Name = "Next Bundle"
	nextBundle.Version = "v2"
	nextBundle.Resources.Agents[0].ID = "agent_next"
	nextBundle = releaseTestBundle(nextBundle)

	snapshot, err := manager.Reload(ctx, nextBundle)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if snapshot.BundleID != "bundle_next" || manager.Snapshot().Version != "v2" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}

	if _, err := manager.RunAgent(ctx, AgentRequest{AgentID: "agent_help", UserMessage: "hello"}); err == nil {
		t.Fatal("expected old agent to be unavailable after reload")
	}
	result, err := manager.RunAgent(ctx, AgentRequest{AgentID: "agent_next", UserMessage: "hello"})
	if err != nil {
		t.Fatalf("run reloaded agent: %v", err)
	}
	if result.Text != "answer from bundle_next" {
		t.Fatalf("unexpected result after reload: %q", result.Text)
	}
}

func releaseTestBundle(bundle coreconfig.BundleSpec) coreconfig.BundleSpec {
	if bundle.Metadata == nil {
		bundle.Metadata = map[string]any{}
	}
	bundle.Metadata[coreconfig.BundleMetadataLifecycle] = string(coreconfig.BundleLifecycleRelease)
	return bundle
}
