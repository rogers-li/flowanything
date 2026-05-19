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

type FileRunHistoryStore struct {
	Path string
	mu   sync.Mutex
}

func NewFileRunHistoryStore(path string) *FileRunHistoryStore {
	return &FileRunHistoryStore{Path: path}
}

func (s *FileRunHistoryStore) SaveRun(_ context.Context, record app.RunRecord) error {
	if record.ID == "" {
		return fmt.Errorf("run id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return err
	}
	upsertRunRecord(&state, record)
	return s.save(state)
}

func (s *FileRunHistoryStore) LoadRun(_ context.Context, id string) (app.RunRecord, error) {
	if id == "" {
		return app.RunRecord{}, fmt.Errorf("run id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return app.RunRecord{}, err
	}
	for _, record := range state.Runs {
		if record.ID == id {
			return record, nil
		}
	}
	return app.RunRecord{}, fmt.Errorf("run %q not found", id)
}

func (s *FileRunHistoryStore) ListRuns(_ context.Context) ([]app.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return nil, err
	}
	return append([]app.RunRecord(nil), state.Runs...), nil
}

type runHistoryFileState struct {
	Runs []app.RunRecord `json:"runs"`
}

func (s *FileRunHistoryStore) load() (runHistoryFileState, error) {
	if s.Path == "" {
		return runHistoryFileState{}, fmt.Errorf("run history store path is required")
	}
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return runHistoryFileState{}, nil
	}
	if err != nil {
		return runHistoryFileState{}, err
	}
	if len(data) == 0 {
		return runHistoryFileState{}, nil
	}
	var state runHistoryFileState
	if err := json.Unmarshal(data, &state); err != nil {
		return runHistoryFileState{}, err
	}
	return state, nil
}

func (s *FileRunHistoryStore) save(state runHistoryFileState) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o644)
}

func upsertRunRecord(state *runHistoryFileState, record app.RunRecord) {
	for index, existing := range state.Runs {
		if existing.ID == record.ID {
			state.Runs[index] = record
			return
		}
	}
	state.Runs = append(state.Runs, record)
}
