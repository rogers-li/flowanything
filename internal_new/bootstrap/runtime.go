package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"flow-anything/core/agentcore"
	coreconfig "flow-anything/core/config"
	coretrace "flow-anything/core/trace"
	configadapter "flow-anything/internal_new/adapters/config"
	connectoradapter "flow-anything/internal_new/adapters/connector"
	modeladapter "flow-anything/internal_new/adapters/model"
	runtimeadapter "flow-anything/internal_new/adapters/runtime"
	"flow-anything/internal_new/app"
	httpapi "flow-anything/internal_new/interfaces/http"
	"flow-anything/internal_new/platformconfig"
)

type RuntimeConfig struct {
	BundlePath        string
	DraftBundlePath   string
	PreviewBundlePath string
	ReleaseBundlePath string
	BundleID          string
	BundleStore       configadapter.BundleStore
	BundleStores      configadapter.BundleStores
	DebugSessionPath  string
	RunHistoryPath    string
	TraceStorePath    string
	Addr              string

	ModelProvider string
	ModelBaseURL  string
	ModelAPIKey   string
	MockContent   string
}

type Runtime struct {
	Host          *app.Host
	Manager       *app.RuntimeManager
	DebugSessions *app.DebugSessionManager
	RunHistory    *app.RunHistory
	ConfigService *platformconfig.Service
	Handler       http.Handler
}

// NewRuntime builds the new backend runtime from a config-as-code bundle.
func NewRuntime(config RuntimeConfig) (Runtime, error) {
	ctx := context.Background()
	stores, err := bundleStoresFor(config)
	if err != nil {
		return Runtime{}, err
	}
	bundle, err := stores.Releases.LoadBundle(ctx, config.BundleID)
	if err != nil {
		return Runtime{}, err
	}
	configService, err := platformconfig.NewServiceWithStores(stores)
	if err != nil {
		return Runtime{}, err
	}
	traceStore := traceStoreFor(config)
	hostFactory := func(ctx context.Context, bundle coreconfig.BundleSpec) (*app.Host, error) {
		return buildRuntimeHost(config, bundle, traceStore)
	}
	host, err := hostFactory(ctx, bundle)
	if err != nil {
		return Runtime{}, err
	}
	manager, err := app.NewRuntimeManager(host, bundle, hostFactory)
	if err != nil {
		return Runtime{}, err
	}
	debugSessions, err := app.NewDebugSessionManager(
		hostFactory,
		app.WithDebugBundleLoader(configService.GetPreviewBundle),
		app.WithDebugSessionStore(debugSessionStoreFor(config)),
	)
	if err != nil {
		return Runtime{}, err
	}
	runHistory := app.NewRunHistoryWithStore(runHistoryStoreFor(config))
	handler := httpapi.NewServer(
		manager,
		httpapi.WithPlatformConfig(configService),
		httpapi.WithRuntimeManager(manager),
		httpapi.WithDebugSessions(debugSessions),
		httpapi.WithRunHistory(runHistory),
	).Handler()
	return Runtime{Host: host, Manager: manager, DebugSessions: debugSessions, RunHistory: runHistory, ConfigService: configService, Handler: handler}, nil
}

func debugSessionStoreFor(config RuntimeConfig) app.DebugSessionStore {
	if config.DebugSessionPath != "" {
		return runtimeadapter.NewFileDebugSessionStore(config.DebugSessionPath)
	}
	return app.NewMemoryDebugSessionStore()
}

func runHistoryStoreFor(config RuntimeConfig) app.RunHistoryStore {
	if config.RunHistoryPath != "" {
		return runtimeadapter.NewFileRunHistoryStore(config.RunHistoryPath)
	}
	return app.NewMemoryRunHistoryStore()
}

func traceStoreFor(config RuntimeConfig) coretrace.Store {
	if config.TraceStorePath != "" {
		return coretrace.NewFileStore(config.TraceStorePath)
	}
	return coretrace.NewMemoryStore()
}

func bundleStoresFor(config RuntimeConfig) (configadapter.BundleStores, error) {
	if config.BundleStores.Drafts != nil || config.BundleStores.Previews != nil || config.BundleStores.Releases != nil {
		if err := config.BundleStores.Validate(); err != nil {
			return configadapter.BundleStores{}, err
		}
		return config.BundleStores, nil
	}
	if config.BundleStore != nil {
		return configadapter.BundleStores{
			Drafts:   config.BundleStore,
			Previews: config.BundleStore,
			Releases: config.BundleStore,
		}, nil
	}
	if config.DraftBundlePath != "" || config.PreviewBundlePath != "" || config.ReleaseBundlePath != "" {
		if config.DraftBundlePath == "" || config.PreviewBundlePath == "" || config.ReleaseBundlePath == "" {
			return configadapter.BundleStores{}, fmt.Errorf("draft, preview, and release bundle paths must be set together")
		}
		return configadapter.BundleStores{
			Drafts:   configadapter.NewFileBundleStore(config.DraftBundlePath),
			Previews: configadapter.NewFileBundleStore(config.PreviewBundlePath),
			Releases: configadapter.NewFileBundleStore(config.ReleaseBundlePath),
		}, nil
	}
	if config.BundlePath == "" {
		return configadapter.BundleStores{}, fmt.Errorf("bundle path or bundle store is required")
	}
	store := configadapter.NewFileBundleStore(config.BundlePath)
	return configadapter.BundleStores{
		Drafts:   store,
		Previews: store,
		Releases: store,
	}, nil
}

func buildRuntimeHost(config RuntimeConfig, bundle coreconfig.BundleSpec, traceStore coretrace.Store) (*app.Host, error) {
	modelClient := modelClientFor(config, bundle)
	return app.NewHost(
		bundle,
		modelClient,
		app.WithTraceStore(traceStore),
		app.WithConnectorProtocolExecutor(connectoradapter.HTTPProtocolExecutor{
			SecretResolver: connectoradapter.EnvSecretResolver{},
		}),
	)
}

// RunHTTP starts the new runtime HTTP server. It is ready for future cmd/*
// switch-over but intentionally not wired into existing commands yet.
func RunHTTP(ctx context.Context, config RuntimeConfig) error {
	runtime, err := NewRuntime(config)
	if err != nil {
		return err
	}
	addr := config.Addr
	if addr == "" {
		addr = ":8081"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           runtime.Handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func modelClientFor(config RuntimeConfig, bundle coreconfig.BundleSpec) agentcore.ModelClient {
	provider := resolveModelProvider(config, bundle)
	if provider == "" || provider == "mock" {
		return modeladapter.MockClient{Content: config.MockContent}
	}
	return modeladapter.OpenAICompatibleClient{
		Provider: provider,
		BaseURL:  resolveModelBaseURL(provider, config.ModelBaseURL),
		APIKey:   resolveModelAPIKey(provider, config.ModelAPIKey),
	}
}

func resolveModelProvider(config RuntimeConfig, bundle coreconfig.BundleSpec) string {
	provider := strings.ToLower(strings.TrimSpace(config.ModelProvider))
	if provider != "" {
		return provider
	}
	return strings.ToLower(strings.TrimSpace(firstModelProvider(bundle)))
}

func resolveModelBaseURL(provider string, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "deepseek":
		return "https://api.deepseek.com"
	case "openai", "openai-compatible":
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}

func resolveModelAPIKey(provider string, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "deepseek":
		return strings.TrimSpace(envValue("DEEPSEEK_API_KEY"))
	case "openai", "openai-compatible":
		return strings.TrimSpace(envValue("OPENAI_API_KEY"))
	default:
		return ""
	}
}

func firstModelProvider(bundle coreconfig.BundleSpec) string {
	for _, model := range bundle.Resources.Models {
		if model.Provider != "" {
			return model.Provider
		}
	}
	return ""
}
