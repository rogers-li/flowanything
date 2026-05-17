package workflow

import (
	"context"
	"time"

	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type Runner interface {
	Run(ctx context.Context, req tool.WorkflowRunRequest) (tool.BackendResult, error)
}

type Adapter struct {
	runner Runner
}

func New(runner Runner) *Adapter {
	return &Adapter{runner: runner}
}

func (a *Adapter) Supports(kind tool.ImplementationType) bool {
	return kind == tool.ImplementationWorkflow
}

func (a *Adapter) Execute(ctx context.Context, spec tool.Spec, call tool.Call) (tool.Result, error) {
	startedAt := time.Now().UTC()
	if a.runner == nil {
		return tool.Result{}, apperrors.New(apperrors.CodeUnavailable, "workflow runner is not configured")
	}
	if spec.Binding.WorkflowID.Empty() {
		return tool.Result{}, apperrors.New(apperrors.CodeInvalidArgument, "workflow binding is required")
	}

	resp, err := a.runner.Run(ctx, tool.WorkflowRunRequest{
		ID:         id.New("wfrun"),
		TenantID:   call.TenantID,
		ToolID:     spec.ID,
		WorkflowID: spec.Binding.WorkflowID,
		Args:       call.Args,
		TraceID:    call.TraceID,
	})
	if err != nil {
		return tool.Result{}, err
	}

	return backendResultToToolResult(call.ID, spec.ID, startedAt, resp), nil
}

func backendResultToToolResult(callID id.ID, toolID id.ID, fallbackStartedAt time.Time, resp tool.BackendResult) tool.Result {
	startedAt := resp.StartedAt
	if startedAt.IsZero() {
		startedAt = fallbackStartedAt
	}
	finishedAt := resp.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}

	return tool.Result{
		CallID:           callID,
		ToolID:           toolID,
		Success:          resp.Success,
		Data:             resp.Data,
		ErrorCode:        resp.ErrorCode,
		ErrorReason:      resp.ErrorReason,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
		WorkflowRun:      resp.WorkflowRun,
		WorkflowNodeRuns: resp.WorkflowNodeRuns,
	}
}
