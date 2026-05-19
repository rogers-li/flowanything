package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	coreconfig "flow-anything/core/config"
	coretrace "flow-anything/core/trace"
)

type HostFactory func(ctx context.Context, bundle coreconfig.BundleSpec) (*Host, error)

type RuntimeSnapshot struct {
	BundleID       string                     `json:"bundle_id"`
	Name           string                     `json:"name,omitempty"`
	Version        string                     `json:"version"`
	Lifecycle      coreconfig.BundleLifecycle `json:"lifecycle,omitempty"`
	SourceBundleID string                     `json:"source_bundle_id,omitempty"`
	ContentHash    string                     `json:"content_hash,omitempty"`
	LoadedAt       time.Time                  `json:"loaded_at"`
}

// RuntimeManager owns the active runtime Host.
//
// Reload builds a brand-new Host first and swaps it in only after construction
// succeeds. In-flight requests keep using the old Host instance they already
// captured; new requests see the newly loaded Bundle.
type RuntimeManager struct {
	mu       sync.RWMutex
	host     *Host
	snapshot RuntimeSnapshot
	factory  HostFactory
	nowFn    func() time.Time
}

func NewRuntimeManager(initialHost *Host, initialBundle coreconfig.BundleSpec, factory HostFactory) (*RuntimeManager, error) {
	if initialHost == nil {
		return nil, fmt.Errorf("initial host is required")
	}
	if factory == nil {
		return nil, fmt.Errorf("host factory is required")
	}
	if err := requireReleaseBundle(initialBundle); err != nil {
		return nil, err
	}
	return &RuntimeManager{
		host:     initialHost,
		snapshot: snapshotFromBundle(initialBundle, time.Now()),
		factory:  factory,
		nowFn:    time.Now,
	}, nil
}

func (m *RuntimeManager) Snapshot() RuntimeSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot
}

func (m *RuntimeManager) Reload(ctx context.Context, bundle coreconfig.BundleSpec) (RuntimeSnapshot, error) {
	if err := requireReleaseBundle(bundle); err != nil {
		return RuntimeSnapshot{}, err
	}
	host, err := m.factory(ctx, bundle)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	snapshot := snapshotFromBundle(bundle, m.now())
	m.mu.Lock()
	m.host = host
	m.snapshot = snapshot
	m.mu.Unlock()
	return snapshot, nil
}

func requireReleaseBundle(bundle coreconfig.BundleSpec) error {
	lifecycle := coreconfig.BundleLifecycle(metadataString(bundle.Metadata, coreconfig.BundleMetadataLifecycle))
	if lifecycle != coreconfig.BundleLifecycleRelease {
		if lifecycle == "" {
			lifecycle = coreconfig.BundleLifecycleDraft
		}
		return fmt.Errorf("runtime can only load release bundles: bundle %q is %q", bundle.ID, lifecycle)
	}
	return nil
}

func (m *RuntimeManager) Catalog() coreconfig.RuntimeCatalog {
	return m.currentHost().Catalog()
}

func (m *RuntimeManager) TraceStore() coretrace.Store {
	return m.currentHost().TraceStore()
}

func (m *RuntimeManager) RunAgent(ctx context.Context, req AgentRequest) (AgentResult, error) {
	return m.currentHost().RunAgent(ctx, req)
}

func (m *RuntimeManager) InvokeTool(ctx context.Context, req ToolRequest) (ToolResult, error) {
	return m.currentHost().InvokeTool(ctx, req)
}

func (m *RuntimeManager) InvokeConnector(ctx context.Context, req ConnectorRequest) (ConnectorResult, error) {
	return m.currentHost().InvokeConnector(ctx, req)
}

func (m *RuntimeManager) RunWorkflow(ctx context.Context, req WorkflowRequest) (WorkflowResult, error) {
	return m.currentHost().RunWorkflow(ctx, req)
}

func (m *RuntimeManager) RunAgentGraph(ctx context.Context, req AgentGraphRequest) (AgentGraphResult, error) {
	return m.currentHost().RunAgentGraph(ctx, req)
}

func (m *RuntimeManager) currentHost() *Host {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.host
}

func (m *RuntimeManager) now() time.Time {
	if m.nowFn != nil {
		return m.nowFn()
	}
	return time.Now()
}

func snapshotFromBundle(bundle coreconfig.BundleSpec, loadedAt time.Time) RuntimeSnapshot {
	return RuntimeSnapshot{
		BundleID:       bundle.ID,
		Name:           bundle.Name,
		Version:        bundle.Version,
		Lifecycle:      coreconfig.BundleLifecycle(metadataString(bundle.Metadata, coreconfig.BundleMetadataLifecycle)),
		SourceBundleID: metadataString(bundle.Metadata, coreconfig.BundleMetadataSourceBundleID),
		ContentHash:    metadataString(bundle.Metadata, coreconfig.BundleMetadataContentHash),
		LoadedAt:       loadedAt.UTC(),
	}
}
