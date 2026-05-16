package domain

import (
	"strings"

	"flow-anything/internal/platform/contracts/knowledge"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateKnowledgeBase(base knowledge.Base) error {
	if base.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if strings.TrimSpace(base.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "knowledge base name is required")
	}
	switch base.Status {
	case "", knowledge.BaseStatusDraft, knowledge.BaseStatusEnabled, knowledge.BaseStatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported knowledge base status")
	}
	return nil
}

func ValidateQuery(query knowledge.Query) error {
	if query.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if strings.TrimSpace(query.Text) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "query text is required")
	}
	if query.TopK < 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "top_k cannot be negative")
	}
	return nil
}

func ValidateDocument(document knowledge.Document) error {
	if document.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if document.KBID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "kb_id is required")
	}
	if strings.TrimSpace(document.Title) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "document title is required")
	}
	if strings.TrimSpace(document.Text) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "document text is required")
	}
	return nil
}
