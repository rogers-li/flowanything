package domain

import (
	"strings"

	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateTool(spec tool.Spec) error {
	if spec.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "tool name is required")
	}
	if spec.Implementation == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "tool implementation is required")
	}
	switch spec.Status {
	case "", tool.StatusDraft, tool.StatusEnabled, tool.StatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported tool status")
	}
	switch spec.SideEffect {
	case "", tool.SideEffectNone, tool.SideEffectRead, tool.SideEffectWrite:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported tool side effect")
	}
	switch spec.RiskLevel {
	case "", tool.RiskLow, tool.RiskMedium, tool.RiskHigh:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported tool risk level")
	}
	if spec.RetryPolicy.MaxAttempts < 0 || spec.RetryPolicy.BackoffMillis < 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "tool retry policy cannot be negative")
	}

	switch spec.Implementation {
	case tool.ImplementationConnector:
		if spec.Binding.ConnectorOperationID.Empty() {
			return apperrors.New(apperrors.CodeInvalidArgument, "connector_operation_id is required for connector tool")
		}
	case tool.ImplementationKnowledge:
		if len(spec.Binding.KnowledgeBaseIDs) == 0 {
			return apperrors.New(apperrors.CodeInvalidArgument, "knowledge_base_ids is required for knowledge tool")
		}
	case tool.ImplementationPython:
		if spec.Binding.PythonPackageID.Empty() {
			return apperrors.New(apperrors.CodeInvalidArgument, "python_package_id is required for python tool")
		}
	case tool.ImplementationMCP:
		if spec.Binding.MCPServerID.Empty() || strings.TrimSpace(spec.Binding.MCPToolName) == "" {
			return apperrors.New(apperrors.CodeInvalidArgument, "mcp_server_id and mcp_tool_name are required for mcp tool")
		}
	case tool.ImplementationWorkflow:
		if spec.Binding.WorkflowID.Empty() {
			return apperrors.New(apperrors.CodeInvalidArgument, "workflow_id is required for workflow tool")
		}
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported tool implementation")
	}

	return nil
}
