package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/agentflow/domain"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryRunStore struct {
	mu       sync.RWMutex
	runs     map[string]domain.FlowRun
	nodeRuns map[string]domain.NodeRun
}

func NewMemoryRunStore() *MemoryRunStore {
	return &MemoryRunStore{
		runs:     map[string]domain.FlowRun{},
		nodeRuns: map[string]domain.NodeRun{},
	}
}

func (s *MemoryRunStore) CreateRun(ctx context.Context, run domain.FlowRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[key(run.TenantID, run.ID)] = run
	return nil
}

func (s *MemoryRunStore) UpdateRun(ctx context.Context, run domain.FlowRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	runKey := key(run.TenantID, run.ID)
	if _, ok := s.runs[runKey]; !ok {
		return apperrors.New(apperrors.CodeNotFound, "agent flow run not found")
	}
	s.runs[runKey] = run
	return nil
}

func (s *MemoryRunStore) GetRun(ctx context.Context, tenantID tenant.ID, runID id.ID) (domain.FlowRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[key(tenantID, runID)]
	if !ok {
		return domain.FlowRun{}, apperrors.New(apperrors.CodeNotFound, "agent flow run not found")
	}
	return run, nil
}

func (s *MemoryRunStore) RecordNodeRun(ctx context.Context, nodeRun domain.NodeRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodeRuns[key(nodeRun.TenantID, nodeRun.ID)] = nodeRun
	return nil
}

func (s *MemoryRunStore) ListNodeRuns(ctx context.Context, tenantID tenant.ID, runID id.ID) ([]domain.NodeRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []domain.NodeRun{}
	for _, nodeRun := range s.nodeRuns {
		if nodeRun.TenantID == tenantID && nodeRun.RunID == runID {
			result = append(result, nodeRun)
		}
	}
	return result, nil
}

func key(tenantID tenant.ID, valueID id.ID) string {
	return tenantID.String() + ":" + valueID.String()
}
