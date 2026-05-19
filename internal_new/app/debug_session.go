package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	coreconfig "flow-anything/core/config"
)

type DebugEntrypoint = coreconfig.BundleEntrypoint

type DebugSessionSnapshot struct {
	ID             string                     `json:"id"`
	BundleID       string                     `json:"bundle_id"`
	SourceBundleID string                     `json:"source_bundle_id,omitempty"`
	Lifecycle      coreconfig.BundleLifecycle `json:"lifecycle"`
	Version        string                     `json:"version"`
	ContentHash    string                     `json:"content_hash,omitempty"`
	Entrypoint     DebugEntrypoint            `json:"entrypoint"`
	CreatedAt      time.Time                  `json:"created_at"`
	LastUsedAt     time.Time                  `json:"last_used_at"`
}

type debugSessionRecord struct {
	snapshot DebugSessionSnapshot
	host     *Host
}

type DebugSessionStore interface {
	SaveSession(ctx context.Context, snapshot DebugSessionSnapshot) error
	LoadSession(ctx context.Context, id string) (DebugSessionSnapshot, error)
	DeleteSession(ctx context.Context, id string) error
	ListSessions(ctx context.Context) ([]DebugSessionSnapshot, error)
}

type MemoryDebugSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]DebugSessionSnapshot
}

func NewMemoryDebugSessionStore() *MemoryDebugSessionStore {
	return &MemoryDebugSessionStore{sessions: map[string]DebugSessionSnapshot{}}
}

func (s *MemoryDebugSessionStore) SaveSession(_ context.Context, snapshot DebugSessionSnapshot) error {
	if snapshot.ID == "" {
		return fmt.Errorf("debug session id is required")
	}
	s.mu.Lock()
	s.sessions[snapshot.ID] = snapshot
	s.mu.Unlock()
	return nil
}

func (s *MemoryDebugSessionStore) LoadSession(_ context.Context, id string) (DebugSessionSnapshot, error) {
	if id == "" {
		return DebugSessionSnapshot{}, fmt.Errorf("debug session id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.sessions[id]
	if !ok {
		return DebugSessionSnapshot{}, fmt.Errorf("debug session %q not found", id)
	}
	return snapshot, nil
}

func (s *MemoryDebugSessionStore) DeleteSession(_ context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("debug session id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("debug session %q not found", id)
	}
	delete(s.sessions, id)
	return nil
}

func (s *MemoryDebugSessionStore) ListSessions(_ context.Context) ([]DebugSessionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DebugSessionSnapshot, 0, len(s.sessions))
	for _, snapshot := range s.sessions {
		out = append(out, snapshot)
	}
	return out, nil
}

type DebugBundleLoader func(ctx context.Context, bundleID string) (coreconfig.BundleSpec, error)

type DebugSessionOption func(*DebugSessionManager)

// DebugSessionManager owns preview runtimes used by Agent Debug and Test Flow.
//
// Each session is bound to one preview Bundle snapshot so multi-turn debugging
// remains deterministic even if the user edits draft configuration afterwards.
type DebugSessionManager struct {
	mu           sync.RWMutex
	factory      HostFactory
	store        DebugSessionStore
	bundleLoader DebugBundleLoader
	hosts        map[string]*Host
	nowFn        func() time.Time
}

func WithDebugSessionStore(store DebugSessionStore) DebugSessionOption {
	return func(manager *DebugSessionManager) {
		manager.store = store
	}
}

func WithDebugBundleLoader(loader DebugBundleLoader) DebugSessionOption {
	return func(manager *DebugSessionManager) {
		manager.bundleLoader = loader
	}
}

func NewDebugSessionManager(factory HostFactory, opts ...DebugSessionOption) (*DebugSessionManager, error) {
	if factory == nil {
		return nil, fmt.Errorf("host factory is required")
	}
	manager := &DebugSessionManager{
		factory: factory,
		store:   NewMemoryDebugSessionStore(),
		hosts:   map[string]*Host{},
		nowFn:   time.Now,
	}
	for _, opt := range opts {
		opt(manager)
	}
	if manager.store == nil {
		return nil, fmt.Errorf("debug session store is required")
	}
	return manager, nil
}

func (m *DebugSessionManager) CreateSession(ctx context.Context, bundle coreconfig.BundleSpec, entrypoint DebugEntrypoint) (DebugSessionSnapshot, error) {
	if entrypoint.ID == "" {
		return DebugSessionSnapshot{}, fmt.Errorf("entrypoint id is required")
	}
	host, err := m.factory(ctx, bundle)
	if err != nil {
		return DebugSessionSnapshot{}, err
	}
	now := m.now().UTC()
	snapshot := DebugSessionSnapshot{
		ID:             newDebugSessionID(),
		BundleID:       bundle.ID,
		SourceBundleID: metadataString(bundle.Metadata, coreconfig.BundleMetadataSourceBundleID),
		Lifecycle:      coreconfig.BundleLifecycle(metadataString(bundle.Metadata, coreconfig.BundleMetadataLifecycle)),
		Version:        bundle.Version,
		ContentHash:    metadataString(bundle.Metadata, coreconfig.BundleMetadataContentHash),
		Entrypoint:     entrypoint,
		CreatedAt:      now,
		LastUsedAt:     now,
	}
	if snapshot.Lifecycle == "" {
		snapshot.Lifecycle = coreconfig.BundleLifecyclePreview
	}
	if err := m.store.SaveSession(ctx, snapshot); err != nil {
		return DebugSessionSnapshot{}, err
	}
	m.mu.Lock()
	m.hosts[snapshot.ID] = host
	m.mu.Unlock()
	return snapshot, nil
}

func (m *DebugSessionManager) ListSessions(ctx context.Context) ([]DebugSessionSnapshot, error) {
	return m.store.ListSessions(ctx)
}

func (m *DebugSessionManager) GetSession(ctx context.Context, id string) (DebugSessionSnapshot, error) {
	return m.store.LoadSession(ctx, id)
}

func (m *DebugSessionManager) DeleteSession(ctx context.Context, id string) error {
	if err := m.store.DeleteSession(ctx, id); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.hosts, id)
	m.mu.Unlock()
	return nil
}

func (m *DebugSessionManager) RunAgent(ctx context.Context, sessionID string, req AgentRequest) (AgentResult, error) {
	record, err := m.touchAndGetRecord(ctx, sessionID)
	if err != nil {
		return AgentResult{}, err
	}
	if req.AgentID == "" {
		if record.snapshot.Entrypoint.Kind != coreconfig.ResourceAgent {
			return AgentResult{}, fmt.Errorf("debug session entrypoint is %s, not agent", record.snapshot.Entrypoint.Kind)
		}
		req.AgentID = record.snapshot.Entrypoint.ID
	}
	return record.host.RunAgent(ctx, req)
}

func (m *DebugSessionManager) RunWorkflow(ctx context.Context, sessionID string, req WorkflowRequest) (WorkflowResult, error) {
	record, err := m.touchAndGetRecord(ctx, sessionID)
	if err != nil {
		return WorkflowResult{}, err
	}
	if req.WorkflowID == "" {
		if record.snapshot.Entrypoint.Kind != coreconfig.ResourceWorkflow {
			return WorkflowResult{}, fmt.Errorf("debug session entrypoint is %s, not workflow", record.snapshot.Entrypoint.Kind)
		}
		req.WorkflowID = record.snapshot.Entrypoint.ID
	}
	return record.host.RunWorkflow(ctx, req)
}

func (m *DebugSessionManager) RunAgentGraph(ctx context.Context, sessionID string, req AgentGraphRequest) (AgentGraphResult, error) {
	record, err := m.touchAndGetRecord(ctx, sessionID)
	if err != nil {
		return AgentGraphResult{}, err
	}
	if req.AgentFlowID == "" {
		if record.snapshot.Entrypoint.Kind != coreconfig.ResourceWorkflow {
			return AgentGraphResult{}, fmt.Errorf("debug session entrypoint is %s, not agent graph", record.snapshot.Entrypoint.Kind)
		}
		req.AgentFlowID = record.snapshot.Entrypoint.ID
	}
	return record.host.RunAgentGraph(ctx, req)
}

func (m *DebugSessionManager) getRecord(ctx context.Context, id string) (debugSessionRecord, error) {
	if id == "" {
		return debugSessionRecord{}, fmt.Errorf("debug session id is required")
	}
	snapshot, err := m.store.LoadSession(ctx, id)
	if err != nil {
		return debugSessionRecord{}, err
	}
	m.mu.RLock()
	host := m.hosts[id]
	m.mu.RUnlock()
	if host == nil {
		if m.bundleLoader == nil {
			return debugSessionRecord{}, fmt.Errorf("debug session %q runtime is not available", id)
		}
		bundle, err := m.bundleLoader(ctx, snapshot.BundleID)
		if err != nil {
			return debugSessionRecord{}, err
		}
		host, err = m.factory(ctx, bundle)
		if err != nil {
			return debugSessionRecord{}, err
		}
		m.mu.Lock()
		m.hosts[id] = host
		m.mu.Unlock()
	}
	return debugSessionRecord{snapshot: snapshot, host: host}, nil
}

func (m *DebugSessionManager) touchAndGetRecord(ctx context.Context, id string) (debugSessionRecord, error) {
	record, err := m.getRecord(ctx, id)
	if err != nil {
		return debugSessionRecord{}, err
	}
	record.snapshot.LastUsedAt = m.now().UTC()
	if err := m.store.SaveSession(ctx, record.snapshot); err != nil {
		return debugSessionRecord{}, err
	}
	return record, nil
}

func (m *DebugSessionManager) now() time.Time {
	if m.nowFn != nil {
		return m.nowFn()
	}
	return time.Now()
}

func newDebugSessionID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "debug_session_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("debug_session_%d", time.Now().UnixNano())
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
