package platformconfig

import (
	"context"
	"strings"
	"testing"

	coreconfig "flow-anything/core/config"
	configadapter "flow-anything/internal_new/adapters/config"
)

func TestCatalogBuildBundleSortsResourcesDeterministically(t *testing.T) {
	catalog := NewCatalog()
	must(t, catalog.UpsertAgent(coreconfig.AgentConfig{
		ResourceMeta: coreconfig.ResourceMeta{ID: "agent_z", Name: "Agent Z"},
		Prompt:       coreconfig.PromptConfig{System: "You are agent z."},
	}))
	must(t, catalog.UpsertAgent(coreconfig.AgentConfig{
		ResourceMeta: coreconfig.ResourceMeta{ID: "agent_a", Name: "Agent A"},
		Prompt:       coreconfig.PromptConfig{System: "You are agent a."},
	}))
	must(t, catalog.UpsertTool(coreconfig.ToolConfig{
		ResourceMeta: coreconfig.ResourceMeta{ID: "tool_z", Name: "Tool Z"},
		Type:         coreconfig.ToolTypeNative,
		Implementation: coreconfig.ToolImplementationSpec{
			Kind: "native",
		},
	}))
	must(t, catalog.UpsertTool(coreconfig.ToolConfig{
		ResourceMeta: coreconfig.ResourceMeta{ID: "tool_a", Name: "Tool A"},
		Type:         coreconfig.ToolTypeNative,
		Implementation: coreconfig.ToolImplementationSpec{
			Kind: "native",
		},
	}))

	bundle, err := catalog.BuildBundle(context.Background(), BundleDraft{
		ID:      "bundle_test",
		Name:    "Test Bundle",
		Version: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := bundle.Resources.Agents[0].ID; got != "agent_a" {
		t.Fatalf("expected sorted agents, got first %q", got)
	}
	if got := bundle.Resources.Tools[0].ID; got != "tool_a" {
		t.Fatalf("expected sorted tools, got first %q", got)
	}
}

func TestPublisherValidatesCompilesAndStoresBundle(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	publisher := NewPublisher(store)
	bundle := publishableBundle()

	result, err := publisher.Publish(context.Background(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.BundleID, "release_bundle_publish_") || result.SourceBundleID != "bundle_publish" || result.Lifecycle != coreconfig.BundleLifecycleRelease || result.ContentHash == "" || result.Counts.Agents != 1 || result.Counts.Models != 1 {
		t.Fatalf("unexpected publish result: %#v", result)
	}

	loaded, err := store.LoadBundle(context.Background(), result.BundleID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Kind != coreconfig.BundleKind {
		t.Fatalf("expected normalized kind, got %q", loaded.Kind)
	}
	if got := loaded.Metadata[coreconfig.BundleMetadataLifecycle]; got != string(coreconfig.BundleLifecycleRelease) {
		t.Fatalf("expected release lifecycle metadata, got %#v", got)
	}
}

func TestPublisherRejectsInvalidReferences(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	publisher := NewPublisher(store)
	bundle := publishableBundle()
	bundle.Resources.Agents[0].Tools = []coreconfig.ResourceBinding{{
		Ref: coreconfig.ResourceRef{Kind: coreconfig.ResourceTool, ID: "tool_missing"},
	}}

	_, err := publisher.Publish(context.Background(), bundle)
	if err == nil {
		t.Fatal("expected publish to reject missing tool reference")
	}
	if !strings.Contains(err.Error(), `referenced tool "tool_missing" does not exist`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceSavesDraftAndValidatesExplicitly(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	draft := coreconfig.BundleSpec{
		ID:      "bundle_draft",
		Name:    "Draft Bundle",
		Version: "draft",
		Resources: coreconfig.ResourceCollection{
			Agents: []coreconfig.AgentConfig{{
				ResourceMeta: coreconfig.ResourceMeta{ID: "agent_incomplete", Name: "Incomplete Agent"},
			}},
		},
	}

	saved, err := service.SaveBundle(context.Background(), draft)
	if err != nil {
		t.Fatalf("save draft should not require runnable config: %v", err)
	}
	if saved.SchemaVersion != coreconfig.SchemaVersionV1 || saved.Kind != coreconfig.BundleKind {
		t.Fatalf("expected draft normalization, got schema=%q kind=%q", saved.SchemaVersion, saved.Kind)
	}

	result := service.ValidateBundle(saved)
	if result.Valid {
		t.Fatal("expected incomplete agent to be invalid at validation time")
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected structured diagnostics")
	}
}

func TestServicePublishesStoredBundle(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveBundle(context.Background(), publishableBundle()); err != nil {
		t.Fatalf("save bundle: %v", err)
	}

	result, err := service.PublishBundle(context.Background(), "bundle_publish")
	if err != nil {
		t.Fatalf("publish bundle: %v", err)
	}
	if !strings.HasPrefix(result.BundleID, "release_bundle_publish_") || result.SourceBundleID != "bundle_publish" || result.Counts.Agents != 1 {
		t.Fatalf("unexpected publish result: %#v", result)
	}

	draft, err := service.GetBundle(context.Background(), "bundle_publish")
	if err != nil {
		t.Fatalf("get source draft after publish: %v", err)
	}
	if draft.ID != "bundle_publish" {
		t.Fatalf("publish should keep source draft unchanged, got %#v", draft.ID)
	}
}

func TestServiceBuildsAndPersistsPreviewBundle(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveBundle(context.Background(), publishableBundle()); err != nil {
		t.Fatalf("save bundle: %v", err)
	}

	preview, info, err := service.BuildPreviewBundle(context.Background(), "bundle_publish", coreconfig.BundleEntrypoint{
		Kind: coreconfig.ResourceAgent,
		ID:   "agent_main",
	})
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}
	if !strings.HasPrefix(preview.ID, "preview_bundle_publish_agent_main_") || info.Lifecycle != coreconfig.BundleLifecyclePreview || info.SourceBundleID != "bundle_publish" || info.Entrypoint.ID != "agent_main" {
		t.Fatalf("unexpected preview info: bundle=%#v info=%#v", preview.ID, info)
	}
	loadedPreview, err := store.LoadBundle(context.Background(), preview.ID)
	if err != nil {
		t.Fatalf("preview bundle should be persisted: %v", err)
	}
	if loadedPreview.Metadata[coreconfig.BundleMetadataLifecycle] != string(coreconfig.BundleLifecyclePreview) {
		t.Fatalf("expected preview lifecycle metadata, got %#v", loadedPreview.Metadata)
	}
}

func TestServiceSeparatesDraftPreviewAndReleaseStores(t *testing.T) {
	stores := configadapter.NewMemoryBundleStores()
	service, err := NewServiceWithStores(stores)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveBundle(context.Background(), publishableBundle()); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if _, err := stores.Releases.LoadBundle(context.Background(), "bundle_publish"); err == nil {
		t.Fatal("draft should not be saved into release store")
	}

	preview, _, err := service.BuildPreviewBundle(context.Background(), "bundle_publish", coreconfig.BundleEntrypoint{
		Kind: coreconfig.ResourceAgent,
		ID:   "agent_main",
	})
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}
	if _, err := stores.Previews.LoadBundle(context.Background(), preview.ID); err != nil {
		t.Fatalf("preview should be saved into preview store: %v", err)
	}
	if _, err := stores.Drafts.LoadBundle(context.Background(), preview.ID); err == nil {
		t.Fatal("preview should not be saved into draft store")
	}

	publish, err := service.PublishBundle(context.Background(), "bundle_publish")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := stores.Releases.LoadBundle(context.Background(), publish.BundleID); err != nil {
		t.Fatalf("release should be saved into release store: %v", err)
	}
	if _, err := stores.Drafts.LoadBundle(context.Background(), publish.BundleID); err == nil {
		t.Fatal("release should not be saved into draft store")
	}
}

func TestServiceUpsertsAndDeletesTopLevelResources(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveBundle(context.Background(), coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		ID:            "bundle_resources",
		Name:          "Resources Bundle",
		Version:       "draft",
	}); err != nil {
		t.Fatalf("save bundle: %v", err)
	}

	bundle, err := service.UpsertResource(context.Background(), "bundle_resources", ResourceDocument{
		Kind: coreconfig.ResourceAgent,
		ID:   "agent_help",
		Resource: coreconfig.AgentConfig{
			ResourceMeta: coreconfig.ResourceMeta{Name: "Help Agent"},
			Prompt:       coreconfig.PromptConfig{System: "You are helpful."},
		},
	})
	if err != nil {
		t.Fatalf("upsert resource: %v", err)
	}
	if len(bundle.Resources.Agents) != 1 || bundle.Resources.Agents[0].ID != "agent_help" {
		t.Fatalf("unexpected bundle agents: %#v", bundle.Resources.Agents)
	}

	resource, err := service.GetResource(context.Background(), "bundle_resources", coreconfig.ResourceAgent, "agent_help")
	if err != nil {
		t.Fatalf("get resource: %v", err)
	}
	if resource.Kind != coreconfig.ResourceAgent || resource.ID != "agent_help" {
		t.Fatalf("unexpected resource: %#v", resource)
	}

	bundle, err = service.DeleteResource(context.Background(), "bundle_resources", coreconfig.ResourceAgent, "agent_help")
	if err != nil {
		t.Fatalf("delete resource: %v", err)
	}
	if len(bundle.Resources.Agents) != 0 {
		t.Fatalf("expected agent to be deleted: %#v", bundle.Resources.Agents)
	}
}

func TestServiceListsResourcesWithKindAndQuery(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveBundle(context.Background(), coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		ID:            "bundle_list_resources",
		Name:          "List Resources Bundle",
		Version:       "draft",
		Resources: coreconfig.ResourceCollection{
			Agents: []coreconfig.AgentConfig{{
				ResourceMeta: coreconfig.ResourceMeta{ID: "agent_sales", Name: "Sales Agent"},
			}},
			Tools: []coreconfig.ToolConfig{{
				ResourceMeta: coreconfig.ResourceMeta{ID: "tool_search", Name: "Search Tool"},
				Type:         coreconfig.ToolTypeNative,
			}},
			Connectors: []coreconfig.ConnectorConfig{{
				ResourceMeta: coreconfig.ResourceMeta{ID: "conn_search", Name: "Search Connector"},
				Protocol:     coreconfig.ConnectorProtocolSpec{Kind: "http", BaseURL: "https://example.com"},
				Operations: []coreconfig.ConnectorOperationConfig{{
					ResourceMeta: coreconfig.ResourceMeta{ID: "op_search", Name: "Search Operation"},
					Request:      coreconfig.ConnectorOperationRequest{Method: "POST", Path: "/search"},
				}},
			}},
		},
	}); err != nil {
		t.Fatalf("save bundle: %v", err)
	}

	tools, err := service.ListResources(context.Background(), "bundle_list_resources", ResourceListFilter{Kind: coreconfig.ResourceTool})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools) != 1 || tools[0].ID != "tool_search" {
		t.Fatalf("unexpected tools: %#v", tools)
	}

	searchResources, err := service.ListResources(context.Background(), "bundle_list_resources", ResourceListFilter{Query: "operation"})
	if err != nil {
		t.Fatalf("list query: %v", err)
	}
	if len(searchResources) != 1 || searchResources[0].Kind != coreconfig.ResourceConnectorOperation || searchResources[0].ParentID != "conn_search" {
		t.Fatalf("unexpected query result: %#v", searchResources)
	}
}

func TestServiceManagesConnectorOperationsInsideConnector(t *testing.T) {
	store := configadapter.NewMemoryBundleStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveBundle(context.Background(), coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		ID:            "bundle_connector_ops",
		Name:          "Connector Ops Bundle",
		Version:       "draft",
		Resources: coreconfig.ResourceCollection{
			Connectors: []coreconfig.ConnectorConfig{{
				ResourceMeta: coreconfig.ResourceMeta{ID: "conn_search", Name: "Search"},
				Protocol:     coreconfig.ConnectorProtocolSpec{Kind: "http", BaseURL: "https://example.com"},
			}},
		},
	}); err != nil {
		t.Fatalf("save bundle: %v", err)
	}

	bundle, err := service.UpsertConnectorOperation(context.Background(), "bundle_connector_ops", "conn_search", coreconfig.ConnectorOperationConfig{
		ResourceMeta: coreconfig.ResourceMeta{ID: "op_search", Name: "Search"},
		Request:      coreconfig.ConnectorOperationRequest{Method: "POST", Path: "/search"},
	})
	if err != nil {
		t.Fatalf("upsert operation: %v", err)
	}
	if got := len(bundle.Resources.Connectors[0].Operations); got != 1 {
		t.Fatalf("expected one operation, got %d", got)
	}

	operation, err := service.GetConnectorOperation(context.Background(), "bundle_connector_ops", "conn_search", "op_search")
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if operation.ID != "op_search" {
		t.Fatalf("unexpected operation: %#v", operation)
	}
	operations, err := service.ListConnectorOperations(context.Background(), "bundle_connector_ops", "conn_search", "search")
	if err != nil {
		t.Fatalf("list operations: %v", err)
	}
	if len(operations) != 1 || operations[0].ID != "op_search" {
		t.Fatalf("unexpected operations: %#v", operations)
	}

	bundle, err = service.DeleteConnectorOperation(context.Background(), "bundle_connector_ops", "conn_search", "op_search")
	if err != nil {
		t.Fatalf("delete operation: %v", err)
	}
	if got := len(bundle.Resources.Connectors[0].Operations); got != 0 {
		t.Fatalf("expected no operations, got %d", got)
	}
}

func publishableBundle() coreconfig.BundleSpec {
	return coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		ID:            "bundle_publish",
		Name:          "Publish Bundle",
		Version:       "2026.05.16-001",
		Resources: coreconfig.ResourceCollection{
			Models: []coreconfig.ModelConfig{{
				ResourceMeta: coreconfig.ResourceMeta{ID: "model_deepseek", Name: "DeepSeek"},
				Provider:     "deepseek",
				Model:        "deepseek-v4-flash",
			}},
			Agents: []coreconfig.AgentConfig{{
				ResourceMeta: coreconfig.ResourceMeta{ID: "agent_main", Name: "Main Agent"},
				Prompt:       coreconfig.PromptConfig{System: "You are a helpful assistant."},
				ModelRef:     coreconfig.ResourceRef{Kind: coreconfig.ResourceModel, ID: "model_deepseek"},
			}},
		},
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
