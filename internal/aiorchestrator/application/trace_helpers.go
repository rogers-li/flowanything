package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"flow-anything/internal/aiorchestrator/domain"
	"flow-anything/internal/contextengine"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/contracts/runtimeevent"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

const maxTracePayloadTextLength = 6000

func (s *Service) GetTrace(ctx context.Context, tenantID tenant.ID, traceID string) (domain.TraceRecord, error) {
	if s.options.TraceStore == nil {
		return domain.TraceRecord{}, apperrors.New(apperrors.CodeUnavailable, "trace store is not configured")
	}
	return s.options.TraceStore.GetTrace(ctx, tenantID, traceID)
}

func (s *Service) startTrace(ctx context.Context, evt event.Event) {
	if s.options.TraceStore == nil || evt.TraceID == "" {
		return
	}

	now := time.Now().UTC()
	record := domain.NewTraceRecord(evt.TraceID, evt.TenantID, evt.AgentID, evt.SessionID, evt.ID, now)
	if err := s.options.TraceStore.StartTrace(ctx, record); err != nil {
		s.logger.Error("failed to start trace", "trace_id", evt.TraceID, "error", err)
		return
	}
	s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeRunStarted, "Agent run started.", map[string]any{
		"event_id":   evt.ID.String(),
		"agent_id":   evt.AgentID.String(),
		"session_id": evt.SessionID.String(),
		"channel":    evt.Channel,
	})
	s.appendTraceStep(ctx, evt, domain.TraceStepEvent, string(evt.Type), domain.TraceStepStatusSucceeded, now, now, map[string]any{
		"event_id":   evt.ID.String(),
		"tenant_id":  evt.TenantID.String(),
		"agent_id":   evt.AgentID.String(),
		"session_id": evt.SessionID.String(),
		"channel":    evt.Channel,
	})
}

func (s *Service) finishTrace(ctx context.Context, evt event.Event, err error) {
	if s.options.TraceStore == nil || evt.TraceID == "" {
		return
	}

	status := domain.TraceStatusSucceeded
	errText := ""
	if err != nil {
		status = domain.TraceStatusFailed
		errText = err.Error()
	}
	if storeErr := s.options.TraceStore.FinishTrace(ctx, evt.TenantID, evt.TraceID, status, errText); storeErr != nil {
		s.logger.Error("failed to finish trace", "trace_id", evt.TraceID, "error", storeErr)
	}
	eventType := runtimeevent.TypeRunCompleted
	message := "Agent run completed."
	if err != nil {
		eventType = runtimeevent.TypeRunFailed
		message = "Agent run failed."
	}
	s.emitRuntimeEvent(ctx, evt, eventType, message, map[string]any{
		"status": status,
		"error":  errText,
	})
}

func (s *Service) appendTraceStep(
	ctx context.Context,
	evt event.Event,
	stepType domain.TraceStepType,
	name string,
	status domain.TraceStepStatus,
	startedAt time.Time,
	finishedAt time.Time,
	metadata map[string]any,
) domain.TraceStep {
	return s.appendTraceStepWithParent(ctx, evt, "", stepType, name, status, startedAt, finishedAt, metadata)
}

func (s *Service) appendTraceStepWithParent(
	ctx context.Context,
	evt event.Event,
	parentID string,
	stepType domain.TraceStepType,
	name string,
	status domain.TraceStepStatus,
	startedAt time.Time,
	finishedAt time.Time,
	metadata map[string]any,
) domain.TraceStep {
	if s.options.TraceStore == nil || evt.TraceID == "" {
		return domain.TraceStep{}
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	if finishedAt.IsZero() {
		finishedAt = startedAt
	}

	step := domain.NewTraceStep(stepType, name, status, startedAt, finishedAt, metadata)
	step.ParentID = parentID
	if err := s.options.TraceStore.AppendStep(ctx, evt.TenantID, evt.TraceID, step); err != nil {
		s.logger.Error("failed to append trace step",
			"trace_id", evt.TraceID,
			"step_type", stepType,
			"step_name", name,
			"error", err,
		)
	}
	s.emitRuntimeStepEvent(ctx, evt, runtimeevent.TypeTraceStepAdded, step.ID, string(step.Type), step.Name, string(step.Status), traceStepMessage(step), map[string]any{
		"step": step,
	})
	return step
}

func traceStepMessage(step domain.TraceStep) string {
	if step.Name == "" {
		return string(step.Type) + " " + string(step.Status)
	}
	return string(step.Type) + " · " + step.Name + " · " + string(step.Status)
}

func (s *Service) recordAgentConfigTrace(ctx context.Context, evt event.Event, config domain.AgentConfig, hasConfig bool) {
	now := time.Now().UTC()
	if !hasConfig {
		s.appendTraceStep(ctx, evt, domain.TraceStepAgent, "default", domain.TraceStepStatusSkipped, now, now, map[string]any{
			"reason": "agent config not loaded",
		})
		return
	}

	s.appendTraceStep(ctx, evt, domain.TraceStepAgent, config.Agent.Name, domain.TraceStepStatusSucceeded, now, now, map[string]any{
		"agent_id":    config.Agent.ID.String(),
		"status":      config.Agent.Status,
		"provider_id": config.Agent.ModelConfig.ProviderID.String(),
		"model":       config.Agent.ModelConfig.Model,
		"skill_count": len(config.Skills),
		"tool_count":  len(config.Tools),
		"tool_ids":    idsToStrings(config.Agent.ToolIDs),
	})
	for _, spec := range config.Skills {
		s.appendTraceStep(ctx, evt, domain.TraceStepSkill, spec.Name, domain.TraceStepStatusSucceeded, now, now, map[string]any{
			"skill_id": spec.ID.String(),
			"status":   spec.Status,
			"tool_ids": idsToStrings(spec.ToolIDs),
		})
	}
}

func (s *Service) recordContextAssembly(ctx context.Context, evt event.Event, report contextengine.Report) {
	now := time.Now().UTC()
	metadata := map[string]any{
		"message_count":          report.MessageCount,
		"estimated_tokens":       report.EstimatedTokens,
		"history_message_count":  report.HistoryMessageCount,
		"included_history_count": report.IncludedHistoryCount,
		"dropped_history_count":  report.DroppedHistoryCount,
		"truncated_count":        report.TruncatedCount,
		"blocks":                 sanitizeValue(contextBlocksMetadata(report.Blocks)),
	}
	s.appendTraceStep(ctx, evt, domain.TraceStepContext, "context_assembled", domain.TraceStepStatusSucceeded, now, now, metadata)
	s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeContextAssembled, "Context assembled.", metadata)
}

func contextBlocksMetadata(blocks []contextengine.BlockReport) []map[string]any {
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, map[string]any{
			"key":              block.Key,
			"type":             block.Type,
			"label":            block.Label,
			"message_count":    block.MessageCount,
			"estimated_tokens": block.EstimatedTokens,
			"included":         block.Included,
			"dropped_reason":   block.DroppedReason,
			"truncated_count":  block.TruncatedCount,
		})
	}
	return result
}

func modelTraceMetadata(req model.ChatRequest, resp model.ChatResponse, phase string, iteration int) map[string]any {
	return map[string]any{
		"phase":           phase,
		"iteration":       iteration,
		"provider":        resp.Provider,
		"provider_url":    resp.ProviderURL,
		"requested_model": req.Model,
		"response_model":  resp.Model,
		"finish_reason":   resp.FinishReason,
		"message_count":   len(req.Messages),
		"tool_count":      len(req.Tools),
		"tool_choice":     req.ToolChoice,
		"tool_call_count": len(resp.Message.ToolCalls),
		"input_tokens":    resp.Usage.InputTokens,
		"output_tokens":   resp.Usage.OutputTokens,
		"total_tokens":    resp.Usage.TotalTokens,
		"request":         compactModelRequest(req),
		"response":        compactModelResponse(resp),
	}
}

func toolTraceMetadata(spec tool.Spec, call tool.Call, result tool.Result, skillRefs []map[string]string) map[string]any {
	return map[string]any{
		"tool_id":                spec.ID.String(),
		"tool_name":              spec.Name,
		"implementation":         spec.Implementation,
		"connector_operation_id": spec.Binding.ConnectorOperationID.String(),
		"risk_level":             spec.RiskLevel,
		"requires_confirmation":  spec.RequiresExecutionConfirmation(),
		"confirmed":              call.Confirmed,
		"arg_keys":               argKeys(call.Args),
		"skills":                 skillRefs,
		"success":                result.Success,
		"error_code":             result.ErrorCode,
		"request": map[string]any{
			"tool_id":   call.ToolID.String(),
			"name":      call.Name,
			"args":      sanitizeValue(call.Args),
			"confirmed": call.Confirmed,
		},
		"response": compactToolResult(result),
	}
}

func connectorTraceMetadata(spec tool.Spec, result tool.Result) map[string]any {
	return map[string]any{
		"connector_operation_id": spec.Binding.ConnectorOperationID.String(),
		"tool_id":                spec.ID.String(),
		"tool_name":              spec.Name,
		"success":                result.Success,
		"error_code":             result.ErrorCode,
		"request": map[string]any{
			"operation_id": spec.Binding.ConnectorOperationID.String(),
			"tool_id":      spec.ID.String(),
			"tool_name":    spec.Name,
		},
		"response": compactToolResult(result),
	}
}

func (s *Service) appendWorkflowTraceSteps(ctx context.Context, evt event.Event, parentID string, spec tool.Spec, result tool.Result) {
	if result.WorkflowRun == nil {
		return
	}

	run := *result.WorkflowRun
	finishedAt := time.Now().UTC()
	if run.FinishedAt != nil {
		finishedAt = *run.FinishedAt
	}
	workflowStep := s.appendTraceStepWithParent(
		ctx,
		evt,
		parentID,
		domain.TraceStepWorkflow,
		workflowTraceName(spec, run),
		workflowRunTraceStatus(run.Status, result.Success),
		run.StartedAt,
		finishedAt,
		workflowRunTraceMetadata(spec, result, run),
	)
	if workflowStep.ID == "" {
		return
	}

	nodeRuns := append([]workflow.NodeRun(nil), result.WorkflowNodeRuns...)
	sort.SliceStable(nodeRuns, func(i, j int) bool {
		left := nodeRuns[i]
		right := nodeRuns[j]
		if !left.StartedAt.Equal(right.StartedAt) {
			return left.StartedAt.Before(right.StartedAt)
		}
		return left.NodeID.String() < right.NodeID.String()
	})
	for _, nodeRun := range nodeRuns {
		nodeFinishedAt := time.Now().UTC()
		if nodeRun.FinishedAt != nil {
			nodeFinishedAt = *nodeRun.FinishedAt
		}
		s.appendTraceStepWithParent(
			ctx,
			evt,
			workflowStep.ID,
			workflowNodeTraceStepType(nodeRun.NodeType),
			workflowNodeTraceName(nodeRun),
			workflowNodeRunTraceStatus(nodeRun.Status),
			nodeRun.StartedAt,
			nodeFinishedAt,
			workflowNodeTraceMetadata(nodeRun),
		)
	}
}

func workflowTraceName(spec tool.Spec, run workflow.Run) string {
	if !run.WorkflowID.Empty() {
		return run.WorkflowID.String()
	}
	if !spec.Binding.WorkflowID.Empty() {
		return spec.Binding.WorkflowID.String()
	}
	return "workflow"
}

func workflowRunTraceStatus(status workflow.RunStatus, success bool) domain.TraceStepStatus {
	switch status {
	case workflow.RunStatusFailed, workflow.RunStatusCanceled:
		return domain.TraceStepStatusFailed
	case workflow.RunStatusPending, workflow.RunStatusRunning:
		return domain.TraceStepStatusStarted
	case workflow.RunStatusSucceeded:
		if success {
			return domain.TraceStepStatusSucceeded
		}
		return domain.TraceStepStatusFailed
	default:
		if success {
			return domain.TraceStepStatusSucceeded
		}
		return domain.TraceStepStatusFailed
	}
}

func workflowNodeRunTraceStatus(status workflow.NodeRunStatus) domain.TraceStepStatus {
	switch status {
	case workflow.NodeRunStatusFailed, workflow.NodeRunStatusCanceled:
		return domain.TraceStepStatusFailed
	case workflow.NodeRunStatusPending, workflow.NodeRunStatusRunning:
		return domain.TraceStepStatusStarted
	case workflow.NodeRunStatusSkipped:
		return domain.TraceStepStatusSkipped
	default:
		return domain.TraceStepStatusSucceeded
	}
}

func workflowNodeTraceStepType(nodeType workflow.NodeType) domain.TraceStepType {
	switch nodeType {
	case workflow.NodeTypeConnectorOperation:
		return domain.TraceStepConnector
	case workflow.NodeTypeTool:
		return domain.TraceStepTool
	case workflow.NodeTypeSkill:
		return domain.TraceStepSkill
	case workflow.NodeTypeAgent:
		return domain.TraceStepAgent
	default:
		return domain.TraceStepEvent
	}
}

func workflowNodeTraceName(nodeRun workflow.NodeRun) string {
	if strings.TrimSpace(nodeRun.NodeName) != "" {
		return nodeRun.NodeName
	}
	if !nodeRun.NodeID.Empty() {
		return nodeRun.NodeID.String()
	}
	return string(nodeRun.NodeType)
}

func workflowRunTraceMetadata(spec tool.Spec, result tool.Result, run workflow.Run) map[string]any {
	return map[string]any{
		"scope":        "workflow",
		"tool_id":      spec.ID.String(),
		"tool_name":    spec.Name,
		"workflow_id":  run.WorkflowID.String(),
		"run_id":       run.ID.String(),
		"run_status":   run.Status,
		"node_count":   len(result.WorkflowNodeRuns),
		"success":      result.Success,
		"error_code":   result.ErrorCode,
		"error_reason": truncateTraceText(result.ErrorReason),
		"request":      sanitizeValue(run.Input),
		"response":     sanitizeValue(run.Output),
		"context":      sanitizeValue(run.Context),
	}
}

func workflowNodeTraceMetadata(nodeRun workflow.NodeRun) map[string]any {
	return map[string]any{
		"scope":       "workflow_node",
		"workflow_id": nodeRun.WorkflowID.String(),
		"run_id":      nodeRun.RunID.String(),
		"node_run_id": nodeRun.ID.String(),
		"node_id":     nodeRun.NodeID.String(),
		"node_type":   nodeRun.NodeType,
		"node_name":   nodeRun.NodeName,
		"request":     sanitizeValue(nodeRun.Input),
		"response":    sanitizeValue(nodeRun.Output),
		"context":     sanitizeValue(nodeRun.Context),
		"error":       truncateTraceText(nodeRun.Error),
	}
}

func toolFailureTraceMetadata(spec tool.Spec, call tool.Call, skillRefs []map[string]string, err error) map[string]any {
	return map[string]any{
		"tool_id":                spec.ID.String(),
		"tool_name":              spec.Name,
		"implementation":         spec.Implementation,
		"connector_operation_id": spec.Binding.ConnectorOperationID.String(),
		"risk_level":             spec.RiskLevel,
		"arg_keys":               argKeys(call.Args),
		"skills":                 skillRefs,
		"error":                  errorText(err),
		"request": map[string]any{
			"tool_id": call.ToolID.String(),
			"name":    call.Name,
			"args":    sanitizeValue(call.Args),
		},
	}
}

func compactModelRequest(req model.ChatRequest) map[string]any {
	return map[string]any{
		"id":          req.ID.String(),
		"tenant_id":   req.TenantID.String(),
		"trace_id":    req.TraceID,
		"model":       req.Model,
		"messages":    sanitizeValue(req.Messages),
		"tools":       compactModelTools(req.Tools),
		"tool_choice": req.ToolChoice,
		"options":     sanitizeValue(req.Options),
	}
}

func compactModelResponse(resp model.ChatResponse) map[string]any {
	return map[string]any{
		"id":            resp.ID.String(),
		"request_id":    resp.RequestID.String(),
		"trace_id":      resp.TraceID,
		"provider":      resp.Provider,
		"provider_url":  resp.ProviderURL,
		"model":         resp.Model,
		"message":       sanitizeValue(resp.Message),
		"finish_reason": resp.FinishReason,
		"usage":         sanitizeValue(resp.Usage),
		"created_at":    resp.CreatedAt,
	}
}

func compactModelTools(tools []model.ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, toolDef := range tools {
		result = append(result, map[string]any{
			"type":        toolDef.Type,
			"name":        toolDef.Function.Name,
			"description": truncateTraceText(toolDef.Function.Description),
			"parameters":  sanitizeValue(toolDef.Function.Parameters),
		})
	}
	return result
}

func compactToolResult(result tool.Result) map[string]any {
	response := map[string]any{
		"call_id":      result.CallID.String(),
		"tool_id":      result.ToolID.String(),
		"success":      result.Success,
		"data":         sanitizeValue(result.Data),
		"error_code":   result.ErrorCode,
		"error_reason": truncateTraceText(result.ErrorReason),
		"started_at":   result.StartedAt,
		"finished_at":  result.FinishedAt,
	}
	if result.WorkflowRun != nil {
		response["workflow_run_id"] = result.WorkflowRun.ID.String()
		response["workflow_id"] = result.WorkflowRun.WorkflowID.String()
		response["workflow_status"] = result.WorkflowRun.Status
		response["workflow_node_count"] = len(result.WorkflowNodeRuns)
	}
	return response
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return truncateTraceText(typed)
	case model.Message:
		return map[string]any{
			"role":         typed.Role,
			"content":      truncateTraceText(typed.Content),
			"tool_calls":   sanitizeValue(typed.ToolCalls),
			"tool_call_id": typed.ToolCallID,
		}
	case []model.Message:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, sanitizeValue(item))
		}
		return result
	case model.ToolCall:
		return map[string]any{
			"id":        typed.ID,
			"type":      typed.Type,
			"function":  typed.Function.Name,
			"arguments": sanitizeValue(typed.Function.Arguments),
		}
	case []model.ToolCall:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, sanitizeValue(item))
		}
		return result
	case model.Options:
		return map[string]any{
			"temperature": typed.Temperature,
			"max_tokens":  typed.MaxTokens,
		}
	case model.Usage:
		return map[string]any{
			"input_tokens":  typed.InputTokens,
			"output_tokens": typed.OutputTokens,
			"total_tokens":  typed.TotalTokens,
		}
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveTraceKey(key) {
				result[key] = "[redacted]"
				continue
			}
			result[key] = sanitizeValue(item)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, sanitizeValue(item))
		}
		return result
	default:
		return typed
	}
}

func truncateTraceText(text string) string {
	if len(text) <= maxTracePayloadTextLength {
		return text
	}
	return text[:maxTracePayloadTextLength] + "...[truncated]"
}

func isSensitiveTraceKey(key string) bool {
	normalized := strings.ToLower(key)
	sensitiveParts := []string{"authorization", "api_key", "apikey", "token", "secret", "password", "credential"}
	for _, part := range sensitiveParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func skillRefsByToolID(skills []skill.Spec) map[string][]map[string]string {
	result := map[string][]map[string]string{}
	for _, spec := range skills {
		for _, toolID := range spec.ToolIDs {
			key := toolID.String()
			result[key] = append(result[key], map[string]string{
				"id":   spec.ID.String(),
				"name": spec.Name,
			})
		}
	}
	return result
}

func idsToStrings(ids []id.ID) []string {
	result := make([]string, 0, len(ids))
	for _, item := range ids {
		result = append(result, item.String())
	}
	return result
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}
