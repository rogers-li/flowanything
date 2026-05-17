package domain

import (
	"net/url"
	"strings"

	"flow-anything/internal/platform/contracts/connector"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateConnector(spec connector.Spec) error {
	if spec.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector name is required")
	}
	if spec.Type == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector type is required")
	}
	if spec.Type != connector.OperationTypeHTTP {
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector type")
	}
	switch spec.Status {
	case "", connector.OperationStatusDraft, connector.OperationStatusEnabled, connector.OperationStatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector status")
	}
	if strings.TrimSpace(spec.BaseURL) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector base_url is required")
	}
	if parsed, err := url.Parse(spec.BaseURL); err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector base_url must be a valid http or https url")
	}
	switch spec.Auth.Type {
	case "", connector.AuthTypeNone, connector.AuthTypeAPIKey, connector.AuthTypeBearer, connector.AuthTypeBasic, connector.AuthTypeOAuth2:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector auth type")
	}

	return nil
}

func ValidateConnectorOperation(spec connector.OperationSpec) error {
	if spec.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector operation name is required")
	}
	if spec.Type == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector operation type is required")
	}
	if spec.Type != connector.OperationTypeHTTP {
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector operation type")
	}
	switch spec.Status {
	case "", connector.OperationStatusDraft, connector.OperationStatusEnabled, connector.OperationStatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector operation status")
	}
	switch spec.ImplementationMode {
	case "", connector.ImplementationModeSimpleHTTP, connector.ImplementationModeTemplateMapping,
		connector.ImplementationModeAdapterService, connector.ImplementationModeWorkflow, connector.ImplementationModeMock:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector implementation mode")
	}
	if spec.ConnectorID.Empty() && strings.TrimSpace(spec.BaseURL) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector operation base_url is required")
	}
	if strings.TrimSpace(spec.BaseURL) != "" {
		if parsed, err := url.Parse(spec.BaseURL); err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return apperrors.New(apperrors.CodeInvalidArgument, "connector operation base_url must be a valid http or https url")
		}
	}
	if strings.TrimSpace(spec.Method) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector operation method is required")
	}
	if strings.TrimSpace(spec.Path) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "connector operation path is required")
	}
	switch spec.Auth.Type {
	case "", connector.AuthTypeNone, connector.AuthTypeAPIKey, connector.AuthTypeBearer, connector.AuthTypeBasic, connector.AuthTypeOAuth2:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector auth type")
	}

	return nil
}
