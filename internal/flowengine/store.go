package flowengine

import (
	"context"
	"sort"
	"sync"

	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type RunStore interface {
	CreateRun(ctx context.Context, run workflow.Run) error
	UpdateRun(ctx context.Context, run workflow.Run) error
	GetRun(ctx context.Context, tenantID tenant.ID, runID id.ID) (workflow.Run, error)
	ListRuns(ctx context.Context, tenantID tenant.ID, workflowID id.ID, limit int) ([]workflow.Run, error)
	RecordNodeRun(ctx context.Context, nodeRun workflow.NodeRun) error
	ListNodeRuns(ctx context.Context, tenantID tenant.ID, runID id.ID) ([]workflow.NodeRun, error)
}

type MemoryRunStore struct {
	mu       sync.RWMutex
	runs     map[string]workflow.Run
	nodeRuns map[string][]workflow.NodeRun
}

func NewMemoryRunStore() *MemoryRunStore {
	return &MemoryRunStore{
		runs:     map[string]workflow.Run{},
		nodeRuns: map[string][]workflow.NodeRun{},
	}
}

func (s *MemoryRunStore) CreateRun(ctx context.Context, run workflow.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[runKey(run.TenantID, run.ID)] = run
	return nil
}

func (s *MemoryRunStore) UpdateRun(ctx context.Context, run workflow.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[runKey(run.TenantID, run.ID)] = run
	return nil
}

func (s *MemoryRunStore) GetRun(ctx context.Context, tenantID tenant.ID, runID id.ID) (workflow.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runKey(tenantID, runID)]
	if !ok {
		return workflow.Run{}, apperrors.New(apperrors.CodeNotFound, "workflow run not found")
	}
	return run, nil
}

func (s *MemoryRunStore) ListRuns(ctx context.Context, tenantID tenant.ID, workflowID id.ID, limit int) ([]workflow.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]workflow.Run, 0)
	for _, run := range s.runs {
		if run.TenantID != tenantID {
			continue
		}
		if !workflowID.Empty() && run.WorkflowID != workflowID {
			continue
		}
		result = append(result, run)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *MemoryRunStore) RecordNodeRun(ctx context.Context, nodeRun workflow.NodeRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := runKey(nodeRun.TenantID, nodeRun.RunID)
	runs := s.nodeRuns[key]
	for i, existing := range runs {
		if existing.ID == nodeRun.ID {
			runs[i] = nodeRun
			s.nodeRuns[key] = runs
			return nil
		}
	}
	s.nodeRuns[key] = append(runs, nodeRun)
	return nil
}

func (s *MemoryRunStore) ListNodeRuns(ctx context.Context, tenantID tenant.ID, runID id.ID) ([]workflow.NodeRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runs := s.nodeRuns[runKey(tenantID, runID)]
	result := make([]workflow.NodeRun, len(runs))
	copy(result, runs)
	return result, nil
}

func runKey(tenantID tenant.ID, runID id.ID) string {
	return tenantID.String() + ":" + runID.String()
}
