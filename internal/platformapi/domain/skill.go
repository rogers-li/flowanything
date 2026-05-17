package domain

import (
	"strings"

	"flow-anything/internal/platform/contracts/skill"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateSkill(spec skill.Spec) error {
	if spec.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "skill name is required")
	}
	switch spec.Status {
	case "", skill.StatusDraft, skill.StatusEnabled, skill.StatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported skill status")
	}
	switch spec.RiskLevel {
	case "", skill.RiskLow, skill.RiskMedium, skill.RiskHigh:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported skill risk level")
	}
	if spec.ExecutionPolicy.MaxToolCalls < 0 || spec.ExecutionPolicy.TimeoutMillis < 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "skill execution policy cannot be negative")
	}

	return nil
}
