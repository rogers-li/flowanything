package runtimeadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"flow-anything/internal_new/app"
)

type FileDebugSessionStore struct {
	Path string
	mu   sync.Mutex
}

func NewFileDebugSessionStore(path string) *FileDebugSessionStore {
	return &FileDebugSessionStore{Path: path}
}

func (s *FileDebugSessionStore) SaveSession(_ context.Context, snapshot app.DebugSessionSnapshot) error {
	if snapshot.ID == "" {
		return fmt.Errorf("debug session id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return err
	}
	upsertDebugSession(&state, snapshot)
	return s.save(state)
}

func (s *FileDebugSessionStore) LoadSession(_ context.Context, id string) (app.DebugSessionSnapshot, error) {
	if id == "" {
		return app.DebugSessionSnapshot{}, fmt.Errorf("debug session id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return app.DebugSessionSnapshot{}, err
	}
	for _, session := range state.Sessions {
		if session.ID == id {
			return session, nil
		}
	}
	return app.DebugSessionSnapshot{}, fmt.Errorf("debug session %q not found", id)
}

func (s *FileDebugSessionStore) DeleteSession(_ context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("debug session id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return err
	}
	next := make([]app.DebugSessionSnapshot, 0, len(state.Sessions))
	found := false
	for _, session := range state.Sessions {
		if session.ID == id {
			found = true
			continue
		}
		next = append(next, session)
	}
	if !found {
		return fmt.Errorf("debug session %q not found", id)
	}
	state.Sessions = next
	return s.save(state)
}

func (s *FileDebugSessionStore) ListSessions(_ context.Context) ([]app.DebugSessionSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return nil, err
	}
	return append([]app.DebugSessionSnapshot(nil), state.Sessions...), nil
}

type debugSessionFileState struct {
	Sessions []app.DebugSessionSnapshot `json:"sessions"`
}

func (s *FileDebugSessionStore) load() (debugSessionFileState, error) {
	if s.Path == "" {
		return debugSessionFileState{}, fmt.Errorf("debug session store path is required")
	}
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return debugSessionFileState{}, nil
	}
	if err != nil {
		return debugSessionFileState{}, err
	}
	if len(data) == 0 {
		return debugSessionFileState{}, nil
	}
	var state debugSessionFileState
	if err := json.Unmarshal(data, &state); err != nil {
		return debugSessionFileState{}, err
	}
	return state, nil
}

func (s *FileDebugSessionStore) save(state debugSessionFileState) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o644)
}

func upsertDebugSession(state *debugSessionFileState, snapshot app.DebugSessionSnapshot) {
	for index, session := range state.Sessions {
		if session.ID == snapshot.ID {
			state.Sessions[index] = snapshot
			return
		}
	}
	state.Sessions = append(state.Sessions, snapshot)
}
