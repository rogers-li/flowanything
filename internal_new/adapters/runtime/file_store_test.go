package runtimeadapter

import (
	"context"
	"testing"
	"time"

	coreconfig "flow-anything/core/config"
	"flow-anything/internal_new/app"
)

func TestFileDebugSessionStorePersistsSessions(t *testing.T) {
	path := t.TempDir() + "/debug_sessions.json"
	store := NewFileDebugSessionStore(path)
	snapshot := app.DebugSessionSnapshot{
		ID:             "debug_session_test",
		BundleID:       "preview_bundle_test",
		SourceBundleID: "bundle_draft",
		Lifecycle:      coreconfig.BundleLifecyclePreview,
		Version:        "draft",
		ContentHash:    "hash",
		Entrypoint:     coreconfig.BundleEntrypoint{Kind: coreconfig.ResourceAgent, ID: "agent_help"},
		CreatedAt:      time.Now().UTC(),
		LastUsedAt:     time.Now().UTC(),
	}
	if err := store.SaveSession(context.Background(), snapshot); err != nil {
		t.Fatalf("save session: %v", err)
	}

	reopened := NewFileDebugSessionStore(path)
	loaded, err := reopened.LoadSession(context.Background(), "debug_session_test")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if loaded.BundleID != "preview_bundle_test" || loaded.Entrypoint.ID != "agent_help" {
		t.Fatalf("unexpected loaded session: %#v", loaded)
	}
	list, err := reopened.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one session, got %#v", list)
	}
}

func TestFileRunHistoryStorePersistsRuns(t *testing.T) {
	path := t.TempDir() + "/run_history.json"
	store := NewFileRunHistoryStore(path)
	record := app.RunRecord{
		ID:              "run_test",
		Type:            app.RunTypeAgent,
		Status:          app.RunStatusSucceeded,
		BundleID:        "preview_bundle_test",
		BundleLifecycle: string(coreconfig.BundleLifecyclePreview),
		AgentRequest: &app.AgentRunHistoryRequest{
			AgentID:     "agent_help",
			UserMessage: "hello",
		},
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Now().UTC(),
	}
	if err := store.SaveRun(context.Background(), record); err != nil {
		t.Fatalf("save run: %v", err)
	}

	reopened := NewFileRunHistoryStore(path)
	loaded, err := reopened.LoadRun(context.Background(), "run_test")
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if loaded.AgentRequest == nil || loaded.AgentRequest.UserMessage != "hello" {
		t.Fatalf("unexpected loaded run: %#v", loaded)
	}
	list, err := reopened.ListRuns(context.Background())
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one run, got %#v", list)
	}
}
