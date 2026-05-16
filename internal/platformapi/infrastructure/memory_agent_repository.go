package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/agent"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryAgentRepository struct {
	mu     sync.RWMutex
	agents map[string]agent.Profile
}

func NewMemoryAgentRepository() *MemoryAgentRepository {
	return &MemoryAgentRepository{
		agents: make(map[string]agent.Profile),
	}
}

func (r *MemoryAgentRepository) SaveAgent(ctx context.Context, profile agent.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents[key(profile.TenantID, profile.ID)] = profile
	return nil
}

func (r *MemoryAgentRepository) GetAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profile, ok := r.agents[key(tenantID, agentID)]
	if !ok {
		return agent.Profile{}, apperrors.New(apperrors.CodeNotFound, "agent not found")
	}

	return profile, nil
}

func (r *MemoryAgentRepository) ListAgents(ctx context.Context, tenantID tenant.ID) ([]agent.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]agent.Profile, 0)
	for _, profile := range r.agents {
		if profile.TenantID == tenantID {
			result = append(result, profile)
		}
	}

	return result, nil
}

func key(tenantID tenant.ID, agentID id.ID) string {
	return tenantID.String() + "/" + agentID.String()
}
