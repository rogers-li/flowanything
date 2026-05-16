package domain

import (
	"strings"

	"flow-anything/internal/flowengine"
	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateWorkflow(spec workflow.Spec) error {
	if err := flowengine.ValidateSpec(spec); err != nil {
		return err
	}
	switch spec.Status {
	case "", workflow.StatusDraft, workflow.StatusEnabled, workflow.StatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported workflow status")
	}
	if strings.TrimSpace(spec.Version) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "workflow version is required")
	}
	return nil
}
