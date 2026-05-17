package workflowruntime

import (
	"context"
	"log/slog"
	"time"

	"flow-anything/internal/flowengine"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Service struct {
	logger   *slog.Logger
	loader   WorkflowLoader
	store    flowengine.RunStore
	executor *flowengine.Executor
}

func NewService(logger *slog.Logger, loader WorkflowLoader, store flowengine.RunStore, registry *flowengine.NodeRegistry) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if store == nil {
		store = flowengine.NewMemoryRunStore()
	}
	return &Service{
		logger:   logger,
		loader:   loader,
		store:    store,
		executor: flowengine.NewExecutor(logger, store, registry),
	}
}

func (s *Service) RunWorkflow(ctx context.Context, req workflow.RunRequest) (workflow.RunResponse, error) {
	spec, directSpec, err := s.workflowSpec(ctx, req)
	if err != nil {
		return workflow.RunResponse{}, err
	}
	if spec.TenantID.Empty() {
		return workflow.RunResponse{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if !directSpec && spec.Status != workflow.StatusEnabled {
		return workflow.RunResponse{}, apperrors.New(apperrors.CodeInvalidArgument, "workflow is not enabled")
	}
	run, err := s.executor.Execute(ctx, spec, req.Input, req.Context, req.TraceID)
	nodeRuns := s.listNodeRuns(ctx, run)
	if err != nil && !run.ID.Empty() {
		return workflow.RunResponse{Run: run, NodeRuns: nodeRuns, Error: err.Error()}, nil
	}
	if err != nil {
		return workflow.RunResponse{}, err
	}
	return workflow.RunResponse{Run: run, NodeRuns: nodeRuns}, nil
}

func (s *Service) workflowSpec(ctx context.Context, req workflow.RunRequest) (workflow.Spec, bool, error) {
	if req.Workflow != nil {
		spec := *req.Workflow
		if spec.TenantID.Empty() {
			spec.TenantID = req.TenantID
		}
		if spec.ID.Empty() {
			spec.ID = req.WorkflowID
		}
		if spec.ID.Empty() {
			spec.ID = id.New("wftest")
		}
		if spec.Name == "" {
			spec.Name = "Workflow Test"
		}
		return spec, true, nil
	}

	if s.loader == nil {
		return workflow.Spec{}, false, apperrors.New(apperrors.CodeUnavailable, "workflow loader is not configured")
	}
	if req.TenantID.Empty() {
		return workflow.Spec{}, false, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if req.WorkflowID.Empty() {
		return workflow.Spec{}, false, apperrors.New(apperrors.CodeInvalidArgument, "workflow_id is required")
	}
	spec, err := s.loader.LoadWorkflow(ctx, req.TenantID, req.WorkflowID)
	return spec, false, err
}

func (s *Service) RunToolWorkflow(ctx context.Context, req tool.WorkflowRunRequest) (tool.BackendResult, error) {
	resp, err := s.RunWorkflow(ctx, workflow.RunRequest{
		ID:         req.ID,
		TenantID:   req.TenantID,
		WorkflowID: req.WorkflowID,
		Input:      req.Args,
		Context:    req.Args,
		TraceID:    req.TraceID,
	})
	if err != nil {
		return tool.BackendResult{}, err
	}
	success := resp.Run.Status == workflow.RunStatusSucceeded && resp.Error == ""
	result := tool.BackendResult{
		RequestID:        req.ID,
		Success:          success,
		Data:             resp.Run.Output,
		StartedAt:        resp.Run.StartedAt,
		FinishedAt:       time.Now().UTC(),
		WorkflowRun:      &resp.Run,
		WorkflowNodeRuns: resp.NodeRuns,
	}
	if resp.Run.FinishedAt != nil {
		result.FinishedAt = *resp.Run.FinishedAt
	}
	if !success {
		result.ErrorCode = "workflow_failed"
		result.ErrorReason = resp.Error
		if result.ErrorReason == "" {
			result.ErrorReason = resp.Run.Error
		}
	}
	return result, nil
}

func (s *Service) GetRun(ctx context.Context, reqTenantID string, runID string) (workflow.Run, error) {
	return s.store.GetRun(ctx, tenant.ID(reqTenantID), id.ID(runID))
}

func (s *Service) GetRunResponse(ctx context.Context, tenantID tenant.ID, runID id.ID) (workflow.RunResponse, error) {
	run, err := s.store.GetRun(ctx, tenantID, runID)
	if err != nil {
		return workflow.RunResponse{}, err
	}
	return workflow.RunResponse{Run: run, NodeRuns: s.listNodeRuns(ctx, run), Error: run.Error}, nil
}

func (s *Service) ListRuns(ctx context.Context, tenantID tenant.ID, workflowID id.ID, limit int) ([]workflow.Run, error) {
	if tenantID.Empty() {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	return s.store.ListRuns(ctx, tenantID, workflowID, limit)
}

func (s *Service) ReplayRun(ctx context.Context, tenantID tenant.ID, runID id.ID, req workflow.ReplayRunRequest) (workflow.RunResponse, error) {
	if tenantID.Empty() {
		tenantID = req.TenantID
	}
	if tenantID.Empty() {
		return workflow.RunResponse{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if runID.Empty() {
		runID = req.RunID
	}
	if runID.Empty() {
		return workflow.RunResponse{}, apperrors.New(apperrors.CodeInvalidArgument, "run_id is required")
	}
	source, err := s.store.GetRun(ctx, tenantID, runID)
	if err != nil {
		return workflow.RunResponse{}, err
	}
	input := source.Input
	if req.Input != nil {
		input = req.Input
	}
	replayContext := source.Context
	if req.Context != nil {
		replayContext = req.Context
	}
	return s.RunWorkflow(ctx, workflow.RunRequest{
		TenantID:   tenantID,
		WorkflowID: source.WorkflowID,
		Input:      input,
		Context:    replayContext,
		TraceID:    req.TraceID,
	})
}

func (s *Service) listNodeRuns(ctx context.Context, run workflow.Run) []workflow.NodeRun {
	if run.ID.Empty() {
		return nil
	}
	nodeRuns, err := s.store.ListNodeRuns(ctx, run.TenantID, run.ID)
	if err != nil {
		return nil
	}
	return nodeRuns
}
