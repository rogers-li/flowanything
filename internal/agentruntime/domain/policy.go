package domain

import (
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

// ValidateExecutionPolicy enforces platform-level execution gates before an
// adapter can perform side effects.
//
// Human confirmation is intentionally disabled for now because the product does
// not yet support an in-conversation confirmation flow. Risk metadata is still
// preserved on tool specs and audit records so the confirmation gate can be
// re-enabled behind an explicit policy when the UX is ready.
func ValidateExecutionPolicy(spec tool.Spec, call tool.Call) error {
	if spec.Status != "" && spec.Status != tool.StatusEnabled {
		return apperrors.New(apperrors.CodeForbidden, "tool is not enabled")
	}
	return nil
}
