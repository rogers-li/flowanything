package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	coreconfig "flow-anything/core/config"
	configadapter "flow-anything/internal_new/adapters/config"
	"flow-anything/internal_new/adapters/model"
	"flow-anything/internal_new/app"
	"flow-anything/internal_new/platformconfig"
)

func TestServerRunAgentAndReadTrace(t *testing.T) {
	host, err := app.NewHost(appTestBundle(), modeladapter.MockClient{Content: "hello via http"})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}
	server := NewServer(host)

	body := bytes.NewBufferString(`{"agent_id":"agent_help","user_message":"hello","trace_id":"trace_http"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/run", body)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result := response["result"].(map[string]any)
	if result["text"] != "hello via http" {
		t.Fatalf("unexpected result: %#v", result)
	}

	traceReq := httptest.NewRequest(http.MethodGet, "/v1/traces/trace_http", nil)
	traceRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(traceRec, traceReq)
	if traceRec.Code != http.StatusOK {
		t.Fatalf("trace status=%d body=%s", traceRec.Code, traceRec.Body.String())
	}
}

func TestServerManagesBundles(t *testing.T) {
	host, err := app.NewHost(appTestBundle(), modeladapter.MockClient{Content: "hello"})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}
	store := configadapter.NewMemoryBundleStore()
	configService, err := platformconfig.NewService(store)
	if err != nil {
		t.Fatalf("new config service: %v", err)
	}
	server := NewServer(host, WithPlatformConfig(configService))

	body, err := json.Marshal(appTestBundle())
	if err != nil {
		t.Fatal(err)
	}
	saveReq := httptest.NewRequest(http.MethodPost, "/v1/bundles", bytes.NewBuffer(body))
	saveRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", saveRec.Code, saveRec.Body.String())
	}

	validateReq := httptest.NewRequest(http.MethodPost, "/v1/bundles/bundle_http_test/validate", nil)
	validateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(validateRec, validateReq)
	if validateRec.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", validateRec.Code, validateRec.Body.String())
	}
	var validation struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal(validateRec.Body.Bytes(), &validation); err != nil {
		t.Fatalf("decode validation: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid bundle: %s", validateRec.Body.String())
	}

	publishReq := httptest.NewRequest(http.MethodPost, "/v1/bundles/bundle_http_test/publish", nil)
	publishRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(publishRec, publishReq)
	if publishRec.Code != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", publishRec.Code, publishRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/bundles/bundle_http_test", nil)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestServerManagesBundleResources(t *testing.T) {
	host, err := app.NewHost(appTestBundle(), modeladapter.MockClient{Content: "hello"})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}
	store := configadapter.NewMemoryBundleStore()
	configService, err := platformconfig.NewService(store)
	if err != nil {
		t.Fatalf("new config service: %v", err)
	}
	if _, err := configService.SaveBundle(context.Background(), coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		ID:            "bundle_resource_http",
		Name:          "Resource HTTP Bundle",
		Version:       "draft",
		Resources: coreconfig.ResourceCollection{
			Connectors: []coreconfig.ConnectorConfig{{
				ResourceMeta: coreconfig.ResourceMeta{ID: "conn_search", Name: "Search"},
				Protocol:     coreconfig.ConnectorProtocolSpec{Kind: "http", BaseURL: "https://example.com"},
			}},
		},
	}); err != nil {
		t.Fatalf("save seed bundle: %v", err)
	}
	server := NewServer(host, WithPlatformConfig(configService))

	agentBody := bytes.NewBufferString(`{"name":"HTTP Agent","prompt":{"system":"You are helpful."}}`)
	upsertReq := httptest.NewRequest(http.MethodPut, "/v1/bundles/bundle_resource_http/resources/agent/agent_http", agentBody)
	upsertRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(upsertRec, upsertReq)
	if upsertRec.Code != http.StatusOK {
		t.Fatalf("upsert resource status=%d body=%s", upsertRec.Code, upsertRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/bundles/bundle_resource_http/resources/agent/agent_http", nil)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get resource status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	listReq := httptest.NewRequest(http.MethodGet, "/v1/bundles/bundle_resource_http/resources/agent?q=http", nil)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list resource status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	operationBody := bytes.NewBufferString(`{"name":"Search","request":{"method":"POST","path":"/search"}}`)
	opReq := httptest.NewRequest(http.MethodPut, "/v1/bundles/bundle_resource_http/connectors/conn_search/operations/op_search", operationBody)
	opRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(opRec, opReq)
	if opRec.Code != http.StatusOK {
		t.Fatalf("upsert operation status=%d body=%s", opRec.Code, opRec.Body.String())
	}
	listOpsReq := httptest.NewRequest(http.MethodGet, "/v1/bundles/bundle_resource_http/connectors/conn_search/operations?q=search", nil)
	listOpsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listOpsRec, listOpsReq)
	if listOpsRec.Code != http.StatusOK {
		t.Fatalf("list operations status=%d body=%s", listOpsRec.Code, listOpsRec.Body.String())
	}

	inspectReq := httptest.NewRequest(http.MethodGet, "/v1/bundles/bundle_resource_http/inspect", nil)
	inspectRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(inspectRec, inspectReq)
	if inspectRec.Code != http.StatusOK {
		t.Fatalf("inspect status=%d body=%s", inspectRec.Code, inspectRec.Body.String())
	}
}

func TestServerReloadsRuntimeBundle(t *testing.T) {
	initialBundle := releaseHTTPBundle(appTestBundle())
	initialHost, err := app.NewHost(initialBundle, modeladapter.MockClient{Content: "runtime initial"})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}
	store := configadapter.NewMemoryBundleStore()
	nextBundle := appTestBundle()
	nextBundle.ID = "bundle_runtime_next"
	nextBundle.Name = "Runtime Next"
	nextBundle.Version = "v2"
	nextBundle.Resources.Agents[0].ID = "agent_next"
	nextBundle = releaseHTTPBundle(nextBundle)
	if err := store.SaveBundle(context.Background(), nextBundle); err != nil {
		t.Fatalf("save next bundle: %v", err)
	}
	configService, err := platformconfig.NewService(store)
	if err != nil {
		t.Fatalf("new config service: %v", err)
	}
	manager, err := app.NewRuntimeManager(
		initialHost,
		initialBundle,
		func(ctx context.Context, bundle coreconfig.BundleSpec) (*app.Host, error) {
			return app.NewHost(bundle, modeladapter.MockClient{Content: "runtime " + bundle.ID})
		},
	)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	server := NewServer(manager, WithPlatformConfig(configService), WithRuntimeManager(manager))

	reloadReq := httptest.NewRequest(http.MethodPost, "/v1/runtime/reload", bytes.NewBufferString(`{"bundle_id":"bundle_runtime_next"}`))
	reloadRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(reloadRec, reloadReq)
	if reloadRec.Code != http.StatusOK {
		t.Fatalf("reload status=%d body=%s", reloadRec.Code, reloadRec.Body.String())
	}

	catalogReq := httptest.NewRequest(http.MethodGet, "/v1/catalog", nil)
	catalogRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(catalogRec, catalogReq)
	if catalogRec.Code != http.StatusOK {
		t.Fatalf("catalog status=%d body=%s", catalogRec.Code, catalogRec.Body.String())
	}
	var catalog coreconfig.BundleSpec
	if err := json.Unmarshal(catalogRec.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if catalog.ID != "bundle_runtime_next" {
		t.Fatalf("expected reloaded catalog, got %q", catalog.ID)
	}

	runReq := httptest.NewRequest(http.MethodPost, "/v1/agents/run", bytes.NewBufferString(`{"agent_id":"agent_next","user_message":"hello"}`))
	runRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("run status=%d body=%s", runRec.Code, runRec.Body.String())
	}
}

func TestServerPublishAndReloadUsesReleaseSnapshot(t *testing.T) {
	initialBundle := releaseHTTPBundle(appTestBundle())
	initialHost, err := app.NewHost(initialBundle, modeladapter.MockClient{Content: "runtime initial"})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}
	store := configadapter.NewMemoryBundleStore()
	nextBundle := appTestBundle()
	nextBundle.ID = "bundle_runtime_draft"
	nextBundle.Name = "Runtime Draft"
	nextBundle.Version = "v2"
	nextBundle.Resources.Agents[0].ID = "agent_release"
	if err := store.SaveBundle(context.Background(), nextBundle); err != nil {
		t.Fatalf("save next bundle: %v", err)
	}
	configService, err := platformconfig.NewService(store)
	if err != nil {
		t.Fatalf("new config service: %v", err)
	}
	manager, err := app.NewRuntimeManager(
		initialHost,
		initialBundle,
		func(ctx context.Context, bundle coreconfig.BundleSpec) (*app.Host, error) {
			return app.NewHost(bundle, modeladapter.MockClient{Content: "release " + bundle.ID})
		},
	)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	server := NewServer(manager, WithPlatformConfig(configService), WithRuntimeManager(manager))

	req := httptest.NewRequest(http.MethodPost, "/v1/bundles/bundle_runtime_draft/publish-and-reload", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish reload status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Publish platformconfig.PublishResult `json:"publish"`
		Runtime app.RuntimeSnapshot          `json:"runtime"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Publish.SourceBundleID != "bundle_runtime_draft" || response.Publish.Lifecycle != coreconfig.BundleLifecycleRelease {
		t.Fatalf("unexpected publish result: %#v", response.Publish)
	}
	if response.Runtime.BundleID != response.Publish.BundleID || response.Runtime.Lifecycle != coreconfig.BundleLifecycleRelease {
		t.Fatalf("runtime should load release snapshot: %#v", response.Runtime)
	}
}

func TestServerCreatesDebugSessionFromPreviewBundle(t *testing.T) {
	host, err := app.NewHost(appTestBundle(), modeladapter.MockClient{Content: "base"})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}
	store := configadapter.NewMemoryBundleStore()
	if err := store.SaveBundle(context.Background(), appTestBundle()); err != nil {
		t.Fatalf("save bundle: %v", err)
	}
	configService, err := platformconfig.NewService(store)
	if err != nil {
		t.Fatalf("new config service: %v", err)
	}
	debugSessions, err := app.NewDebugSessionManager(func(ctx context.Context, bundle coreconfig.BundleSpec) (*app.Host, error) {
		return app.NewHost(bundle, modeladapter.MockClient{Content: "preview " + bundle.ID})
	})
	if err != nil {
		t.Fatalf("new debug sessions: %v", err)
	}
	runHistory := app.NewRunHistory()
	server := NewServer(host, WithPlatformConfig(configService), WithDebugSessions(debugSessions), WithRunHistory(runHistory))

	createBody := bytes.NewBufferString(`{"bundle_id":"bundle_http_test","entrypoint":{"kind":"agent","id":"agent_help"}}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/debug-sessions", createBody)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create debug session status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Session app.DebugSessionSnapshot          `json:"session"`
		Preview platformconfig.BundleSnapshotInfo `json:"preview"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Session.ID == "" || created.Session.Lifecycle != coreconfig.BundleLifecyclePreview || created.Preview.SourceBundleID != "bundle_http_test" {
		t.Fatalf("unexpected debug session: %#v", created)
	}

	runReq := httptest.NewRequest(http.MethodPost, "/v1/debug-sessions/"+created.Session.ID+"/agents/run", bytes.NewBufferString(`{"user_message":"hello","trace_context":{"trace_id":"trace_debug_http"}}`))
	runRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("run debug status=%d body=%s", runRec.Code, runRec.Body.String())
	}
	var runResponse map[string]any
	if err := json.Unmarshal(runRec.Body.Bytes(), &runResponse); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	result := runResponse["result"].(map[string]any)
	if got := result["text"].(string); got == "" || got == "base" {
		t.Fatalf("expected preview runtime result, got %#v", result)
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/v1/run-history", nil)
	historyRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(historyRec, historyReq)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("history status=%d body=%s", historyRec.Code, historyRec.Body.String())
	}
	var history struct {
		Items []app.RunRecord `json:"items"`
	}
	if err := json.Unmarshal(historyRec.Body.Bytes(), &history); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(history.Items) != 1 || history.Items[0].SessionID != created.Session.ID || history.Items[0].BundleLifecycle != string(coreconfig.BundleLifecyclePreview) {
		t.Fatalf("unexpected history: %#v", history.Items)
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/v1/run-history/"+history.Items[0].ID+"/replay", nil)
	replayRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", replayRec.Code, replayRec.Body.String())
	}
}

func appTestBundle() coreconfig.BundleSpec {
	return coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		Kind:          coreconfig.BundleKind,
		ID:            "bundle_http_test",
		Name:          "HTTP Test Bundle",
		Version:       "v1",
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

func releaseHTTPBundle(bundle coreconfig.BundleSpec) coreconfig.BundleSpec {
	if bundle.Metadata == nil {
		bundle.Metadata = map[string]any{}
	}
	bundle.Metadata[coreconfig.BundleMetadataLifecycle] = string(coreconfig.BundleLifecycleRelease)
	return bundle
}
