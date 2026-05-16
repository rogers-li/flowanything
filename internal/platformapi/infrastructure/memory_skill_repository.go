package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/skill"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemorySkillRepository struct {
	mu     sync.RWMutex
	skills map[string]skill.Spec
}

func NewMemorySkillRepository() *MemorySkillRepository {
	return &MemorySkillRepository{
		skills: make(map[string]skill.Spec),
	}
}

func (r *MemorySkillRepository) SaveSkill(ctx context.Context, spec skill.Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills[key(spec.TenantID, spec.ID)] = spec
	return nil
}

func (r *MemorySkillRepository) GetSkill(ctx context.Context, tenantID tenant.ID, skillID id.ID) (skill.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.skills[key(tenantID, skillID)]
	if !ok {
		return skill.Spec{}, apperrors.New(apperrors.CodeNotFound, "skill not found")
	}

	return spec, nil
}

func (r *MemorySkillRepository) ListSkills(ctx context.Context, tenantID tenant.ID) ([]skill.Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]skill.Spec, 0)
	for _, spec := range r.skills {
		if spec.TenantID == tenantID {
			result = append(result, spec)
		}
	}

	return result, nil
}
