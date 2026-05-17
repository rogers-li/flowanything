package domain

import (
	"strings"

	"flow-anything/internal/platform/contracts/model"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateChatRequest(req model.ChatRequest) error {
	if req.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if len(req.Messages) == 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "messages is required")
	}
	for _, message := range req.Messages {
		if message.Role == "" {
			return apperrors.New(apperrors.CodeInvalidArgument, "message role is required")
		}
		if strings.TrimSpace(message.Content) == "" && len(message.ToolCalls) == 0 {
			return apperrors.New(apperrors.CodeInvalidArgument, "message content is required")
		}
	}

	return nil
}
