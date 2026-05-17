package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"flow-anything/internal/agentruntime/domain"
	"flow-anything/internal/agentruntime/ports"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Service struct {
	logger   *slog.Logger
	catalog  ports.ToolCatalog
	recorder ports.ExecutionRecorder
	reader   ports.ExecutionReader
	adapters []ports.ToolAdapter
}

func New(logger *slog.Logger, catalog ports.ToolCatalog, recorder ports.ExecutionRecorder, adapters ...ports.ToolAdapter) *Service {
	service := &Service{
		logger:   logger,
		catalog:  catalog,
		recorder: recorder,
		adapters: adapters,
	}
	if reader, ok := recorder.(ports.ExecutionReader); ok {
		service.reader = reader
	}
	return service
}

// ExecuteTool is the Agent Runtime execution boundary for all tool types.
//
// It resolves the latest tool specification from the catalog, records lifecycle
// events, selects an adapter by implementation type, and normalizes failures
// into a tool.Result so callers can audit both successful and failed execution.
func (s *Service) ExecuteTool(ctx context.Context, call tool.Call) (tool.Result, error) {
	if call.ID.Empty() {
		call.ID = id.New("toolcall")
	}
	startedAt := time.Now().UTC()

	execution, err := domain.NewExecution(call)
	if err != nil {
		return tool.Result{}, err
	}
	record := domain.NewExecutionRecord(execution, tool.Spec{})
	if s.recorder != nil {
		if err := s.recorder.RecordStarted(ctx, record); err != nil {
			return tool.Result{}, err
		}
	}

	if s.catalog == nil {
		return s.finish(ctx, record, execution.Failure("missing_tool_catalog", "tool catalog is not configured"),
			apperrors.New(apperrors.CodeUnavailable, "tool catalog is not configured"))
	}

	spec, err := s.catalog.GetTool(ctx, call)
	if err != nil {
		return s.finish(ctx, record, execution.Failure("tool_not_found", err.Error()), err)
	}
	call.Name = spec.Name
	call.Implementation = spec.Implementation
	execution.Call = call
	record = domain.NewExecutionRecord(execution, spec)
	if s.logger != nil {
		s.logger.Info("tool execution started",
			"call_id", call.ID.String(),
			"tenant_id", call.TenantID.String(),
			"tool_id", call.ToolID.String(),
			"tool_name", spec.Name,
			"implementation", spec.Implementation,
			"timeout_ms", spec.TimeoutMillis,
			"risk_level", spec.RiskLevel,
			"requires_confirmation", spec.RequiresConfirmation,
			"arg_keys", argKeys(call.Args),
			"trace_id", call.TraceID,
		)
	}

	if spec.Implementation == "" {
		return s.finish(ctx, record, execution.Failure("missing_implementation", "tool implementation is not configured"),
			apperrors.New(apperrors.CodeInvalidArgument, "tool implementation is not configured"))
	}
	if err := domain.ValidateExecutionPolicy(spec, call); err != nil {
		return s.finish(ctx, record, execution.Failure("confirmation_required", err.Error()), err)
	}
	if err := domain.ValidateInputSchema(call.Args, spec.InputSchema); err != nil {
		return s.finish(ctx, record, execution.Failure("invalid_arguments", err.Error()), err)
	}

	for _, adapter := range s.adapters {
		if adapter.Supports(spec.Implementation) {
			result, err := s.executeWithTimeout(ctx, adapter, spec, call)
			if err != nil {
				code := "adapter_failed"
				if errors.Is(err, context.DeadlineExceeded) {
					code = "timeout"
				}
				if s.logger != nil {
					s.logger.Error("tool execution failed",
						"call_id", call.ID.String(),
						"tenant_id", call.TenantID.String(),
						"tool_id", call.ToolID.String(),
						"tool_name", spec.Name,
						"implementation", spec.Implementation,
						"timeout_ms", spec.TimeoutMillis,
						"duration_ms", time.Since(startedAt).Milliseconds(),
						"trace_id", call.TraceID,
						"error_code", code,
						"error", err,
					)
				}
				return s.finish(ctx, record, execution.Failure(code, err.Error()), err)
			}
			if s.logger != nil {
				s.logger.Info("tool execution completed",
					"call_id", call.ID.String(),
					"tenant_id", call.TenantID.String(),
					"tool_id", call.ToolID.String(),
					"tool_name", spec.Name,
					"implementation", spec.Implementation,
					"success", result.Success,
					"error_code", result.ErrorCode,
					"duration_ms", time.Since(startedAt).Milliseconds(),
					"trace_id", call.TraceID,
				)
			}
			return s.finish(ctx, record, result, nil)
		}
	}

	s.logger.Info("tool execution skipped because no adapter is registered",
		"call_id", call.ID.String(),
		"tool_id", call.ToolID.String(),
		"implementation", spec.Implementation,
	)

	return s.finish(ctx, record, tool.Result{
		CallID:     call.ID,
		ToolID:     call.ToolID,
		Success:    false,
		ErrorCode:  "no_adapter",
		StartedAt:  execution.StartedAt,
		FinishedAt: time.Now().UTC(),
	}, apperrors.New(apperrors.CodeUnavailable, "no tool adapter registered"))
}

func (s *Service) GetExecution(ctx context.Context, tenantID tenant.ID, callID id.ID) (tool.ExecutionRecord, error) {
	if s.reader == nil {
		return tool.ExecutionRecord{}, apperrors.New(apperrors.CodeUnavailable, "execution reader is not configured")
	}
	if tenantID.Empty() {
		return tool.ExecutionRecord{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if callID.Empty() {
		return tool.ExecutionRecord{}, apperrors.New(apperrors.CodeInvalidArgument, "call_id is required")
	}

	return s.reader.GetExecution(ctx, tenantID, callID)
}

func (s *Service) executeWithTimeout(ctx context.Context, adapter ports.ToolAdapter, spec tool.Spec, call tool.Call) (tool.Result, error) {
	execCtx := ctx
	cancel := func() {}
	if spec.TimeoutMillis > 0 {
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(spec.TimeoutMillis)*time.Millisecond)
	}
	defer cancel()

	result, err := adapter.Execute(execCtx, spec, call)
	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return tool.Result{}, toolTimeoutError()
		}
		return tool.Result{}, err
	}
	if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		return tool.Result{}, toolTimeoutError()
	}

	return result, nil
}

func toolTimeoutError() error {
	return apperrors.Wrap(apperrors.CodeUnavailable, "tool execution timed out", context.DeadlineExceeded)
}

func argKeys(args map[string]any) []string {
	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	return keys
}

func (s *Service) finish(ctx context.Context, record tool.ExecutionRecord, result tool.Result, resultErr error) (tool.Result, error) {
	record = domain.CompleteExecutionRecord(record, result)
	if s.recorder != nil {
		if err := s.recorder.RecordFinished(ctx, record); err != nil {
			return tool.Result{}, err
		}
	}

	if record.Result == nil {
		return result, resultErr
	}
	return *record.Result, resultErr
}
