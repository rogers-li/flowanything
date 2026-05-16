package domain

import (
	"strings"

	"flow-anything/internal/platform/contracts/agent"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateAgent(profile agent.Profile) error {
	if profile.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if strings.TrimSpace(profile.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "agent name is required")
	}
	switch profile.Status {
	case "", agent.StatusDraft, agent.StatusEnabled, agent.StatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported agent status")
	}
	if profile.RuntimePolicy.MaxTurns < 0 || profile.RuntimePolicy.MaxToolCalls < 0 || profile.RuntimePolicy.ResponseTimeoutMs < 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "agent runtime policy cannot be negative")
	}
	if profile.ModelConfig.Temperature < 0 || profile.ModelConfig.Temperature > 2 {
		return apperrors.New(apperrors.CodeInvalidArgument, "agent model temperature must be between 0 and 2")
	}

	return nil
}
