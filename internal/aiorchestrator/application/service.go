package application

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	orchestration "flow-anything/internal/agentorchestration"
	"flow-anything/internal/aiorchestrator/domain"
	"flow-anything/internal/aiorchestrator/ports"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/contracts/runtimeevent"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/trace"
)

type Service struct {
	logger       *slog.Logger
	toolRuntime  ports.ToolRuntime
	modelClient  ports.ModelClient
	configLoader ports.AgentConfigLoader
	options      runtimeOptions
}

const runtimeSystemPromptPayloadKey = "runtime_system_prompt"
const runtimeDisableToolsPayloadKey = orchestration.RuntimeDisableToolsPayloadKey

func New(logger *slog.Logger, toolRuntime ports.ToolRuntime, modelClient ports.ModelClient, configLoader ports.AgentConfigLoader, opts ...Option) *Service {
	options := defaultRuntimeOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	return &Service{
		logger:       logger,
		toolRuntime:  toolRuntime,
		modelClient:  modelClient,
		configLoader: configLoader,
		options:      options,
	}
}

// HandleEvent is the Orchestrator entry point for one committed user turn.
//
// It keeps the public event contract stable while delegating either to an
// explicit tool execution path, useful for deterministic integrations, or to the
// model-driven tool-calling path used by Agents.
func (s *Service) HandleEvent(ctx context.Context, evt event.Event) (resp event.Response, err error) {
	evt = normalizeEvent(ctx, evt)
	s.startTrace(ctx, evt)
	defer func() {
		s.finishTrace(ctx, evt, err)
	}()

	if toolRequest, ok := domain.NewToolRequest(evt.Payload); ok {
		return s.handleToolRequest(ctx, evt, toolRequest)
	}

	reply := defaultFallbackReply
	if s.modelClient != nil {
		modelReply, err := s.generateReply(ctx, evt)
		if err != nil {
			return event.Response{}, err
		}
		reply = modelReply
	} else {
		message := domain.NewUserMessage(textFromPayload(evt.Payload))
		if !message.Empty() {
			reply = defaultFallbackReply
		}
	}

	s.logger.Info("event handled",
		"event_id", evt.ID.String(),
		"trace_id", evt.TraceID,
		"type", evt.Type,
	)

	return event.Response{
		EventID: evt.ID,
		TraceID: evt.TraceID,
		Actions: []event.Action{
			{
				Type: event.ActionSpeak,
				Text: reply,
			},
			{
				Type: event.ActionEndTurn,
			},
		},
	}, nil
}

func (s *Service) generateReply(ctx context.Context, evt event.Event) (string, error) {
	userText := textFromPayload(evt.Payload)
	if userText == "" {
		userText = defaultEmptyUserInputText
	}

	agentConfig, hasAgentConfig, err := s.loadAgentConfig(ctx, evt)
	if err != nil {
		return "", err
	}
	systemPrompt := s.options.DefaultSystemPrompt
	tools := []model.ToolDefinition{}
	toolByName := map[string]tool.Spec{}
	skillByName := map[string]skillExecutionBinding{}
	skillRefsByTool := map[string][]map[string]string{}
	modelName := ""
	modelOptions := model.Options{}
	maxToolIterations := s.options.MaxToolIterations
	skillMode, selectedSkill, skillInput := skillExecutionFromPayload(evt.Payload, agentConfig)
	if hasAgentConfig {
		systemPrompt = agentConfig.SystemPrompt(s.options.DefaultSystemPrompt)
		if !runtimeDisableToolsFromPayload(evt.Payload) {
			runtimeTools := agentConfig.Tools
			runtimeSkills := agentConfig.Skills
			if skillMode {
				runtimeTools = toolsForSkill(agentConfig.Tools, selectedSkill)
				runtimeSkills = []skill.Spec{selectedSkill}
				maxToolIterations = skillMaxToolIterations(selectedSkill, s.options.MaxToolIterations)
			}
			tools = toModelTools(runtimeTools)
			toolByName = mapToolsByName(runtimeTools)
			skillRefsByTool = skillRefsByToolID(runtimeSkills)
			if !skillMode {
				tools = append(tools, toModelSkillTools(agentConfig.Skills)...)
				skillByName = mapSkillExecutionBindings(agentConfig.Skills, agentConfig.Tools)
			}
		}
		modelName = strings.TrimSpace(agentConfig.Agent.ModelConfig.Model)
		modelOptions.Temperature = agentConfig.Agent.ModelConfig.Temperature
		if agentConfig.Agent.RuntimePolicy.MaxToolCalls > 0 && !skillMode {
			maxToolIterations = agentConfig.Agent.RuntimePolicy.MaxToolCalls
		}
	}
	runtimeSystemPrompt := runtimeSystemPromptFromPayload(evt.Payload)
	if skillMode {
		systemPrompt = runtimeSystemPrompt
		if systemPrompt == "" {
			systemPrompt = orchestration.BuildSkillActionSystemPrompt(selectedSkill, orchestration.Action{
				SkillID: selectedSkill.ID,
				Task:    userText,
				Input:   skillInput,
			})
		}
	} else {
		systemPrompt = appendRuntimeSystemPrompt(systemPrompt, runtimeSystemPrompt)
	}

	history, conversationRef, hasConversation, err := s.loadConversationHistory(ctx, evt)
	if err != nil {
		return "", err
	}
	if skillMode {
		history = nil
		hasConversation = false
	}
	s.recordAgentConfigTrace(ctx, evt, agentConfig, hasAgentConfig)
	if skillMode {
		s.recordSkillExecutionTrace(ctx, evt, selectedSkill, tools, maxToolIterations)
	}
	s.logger.Info("agent model context resolved",
		"event_id", evt.ID.String(),
		"trace_id", evt.TraceID,
		"tenant_id", evt.TenantID.String(),
		"agent_id", evt.AgentID.String(),
		"session_id", evt.SessionID.String(),
		"has_agent_config", hasAgentConfig,
		"execution_mode", executionModeName(skillMode),
		"skill_id", selectedSkill.ID.String(),
		"model", modelName,
		"temperature", modelOptions.Temperature,
		"tool_count", len(tools),
		"history_message_count", len(history),
	)
	runResult, err := s.executePromptToolRun(ctx, evt, promptToolExecution{
		SystemPrompt:          systemPrompt,
		UserText:              userText,
		History:               history,
		ModelName:             modelName,
		ModelOptions:          modelOptions,
		Tools:                 tools,
		ToolByName:            toolByName,
		SkillByName:           skillByName,
		SkillRefsByTool:       skillRefsByTool,
		DefaultToolArgsByName: defaultToolArgsForSkill(selectedSkill, skillInput),
		MaxToolIterations:     maxToolIterations,
	})
	if err != nil {
		return "", err
	}
	if hasConversation {
		if err := s.persistCurrentTurn(ctx, conversationRef, runResult.CurrentTurnMessages); err != nil {
			return "", err
		}
	}

	return runResult.Reply, nil
}

func (s *Service) chatModel(ctx context.Context, evt event.Event, phase string, iteration int, req model.ChatRequest) (model.ChatResponse, error) {
	startedAt := time.Now()
	s.logger.Info("model chat started",
		"event_id", evt.ID.String(),
		"trace_id", evt.TraceID,
		"agent_id", evt.AgentID.String(),
		"session_id", evt.SessionID.String(),
		"phase", phase,
		"iteration", iteration,
		"requested_model", req.Model,
		"message_count", len(req.Messages),
		"tool_count", len(req.Tools),
		"tool_choice", req.ToolChoice,
	)
	s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeLLMStarted, "LLM call started.", map[string]any{
		"phase":           phase,
		"iteration":       iteration,
		"requested_model": req.Model,
		"message_count":   len(req.Messages),
		"tool_count":      len(req.Tools),
		"tool_choice":     req.ToolChoice,
	})

	resp, err := s.modelClient.Chat(ctx, req)
	if err != nil {
		s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeLLMFailed, "LLM call failed.", map[string]any{
			"phase":           phase,
			"iteration":       iteration,
			"requested_model": req.Model,
			"error":           err.Error(),
		})
		s.appendTraceStep(
			ctx,
			evt,
			domain.TraceStepModel,
			phase,
			domain.TraceStepStatusFailed,
			startedAt,
			time.Now().UTC(),
			map[string]any{
				"phase":           phase,
				"iteration":       iteration,
				"requested_model": req.Model,
				"message_count":   len(req.Messages),
				"tool_count":      len(req.Tools),
				"tool_choice":     req.ToolChoice,
				"error":           err.Error(),
			},
		)
		s.logger.Error("model chat failed",
			"event_id", evt.ID.String(),
			"trace_id", evt.TraceID,
			"agent_id", evt.AgentID.String(),
			"session_id", evt.SessionID.String(),
			"phase", phase,
			"iteration", iteration,
			"requested_model", req.Model,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return model.ChatResponse{}, err
	}

	s.logger.Info("model chat completed",
		"event_id", evt.ID.String(),
		"trace_id", evt.TraceID,
		"agent_id", evt.AgentID.String(),
		"session_id", evt.SessionID.String(),
		"phase", phase,
		"iteration", iteration,
		"requested_model", req.Model,
		"response_model", resp.Model,
		"finish_reason", resp.FinishReason,
		"tool_call_count", len(resp.Message.ToolCalls),
		"input_tokens", resp.Usage.InputTokens,
		"output_tokens", resp.Usage.OutputTokens,
		"total_tokens", resp.Usage.TotalTokens,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
	s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeLLMCompleted, "LLM call completed.", map[string]any{
		"phase":           phase,
		"iteration":       iteration,
		"requested_model": req.Model,
		"response_model":  resp.Model,
		"finish_reason":   resp.FinishReason,
		"duration_ms":     time.Since(startedAt).Milliseconds(),
		"tool_call_count": len(resp.Message.ToolCalls),
	})
	s.appendTraceStep(
		ctx,
		evt,
		domain.TraceStepModel,
		phase,
		domain.TraceStepStatusSucceeded,
		startedAt,
		time.Now().UTC(),
		modelTraceMetadata(req, resp, phase, iteration),
	)

	return resp, nil
}

func (s *Service) loadConversationHistory(ctx context.Context, evt event.Event) ([]model.Message, domain.ConversationRef, bool, error) {
	ref, ok := domain.NewConversationRef(evt.TenantID, evt.AgentID, evt.SessionID)
	if !ok || s.options.ConversationStore == nil {
		return nil, domain.ConversationRef{}, false, nil
	}

	messages, err := s.options.ConversationStore.LoadMessages(ctx, ref, s.options.MaxHistoryMessages)
	if err != nil {
		return nil, domain.ConversationRef{}, false, err
	}

	return messages, ref, true, nil
}

func (s *Service) persistCurrentTurn(ctx context.Context, ref domain.ConversationRef, messages []model.Message) error {
	if s.options.ConversationStore == nil || len(messages) == 0 {
		return nil
	}

	return s.options.ConversationStore.AppendMessages(ctx, ref, messages)
}

// executeToolCalls appends the assistant tool-call message and the matching
// tool result messages to the conversation. The method is intentionally
// side-effect aware: it is the only place in the Orchestrator application layer
// that invokes Agent Runtime during model-driven execution.
type toolExecutionContext struct {
	ToolByName            map[string]tool.Spec
	SkillByName           map[string]skillExecutionBinding
	SkillRefsByTool       map[string][]map[string]string
	DefaultToolArgsByName map[string]map[string]any
	ModelName             string
	ModelOptions          model.Options
}

func (s *Service) executeToolCalls(ctx context.Context, evt event.Event, messages []model.Message, assistantMessage model.Message, execution toolExecutionContext) ([]model.Message, error) {
	if s.toolRuntime == nil {
		return nil, apperrors.New(apperrors.CodeUnavailable, "tool runtime is not configured")
	}

	messages = append(messages, assistantMessage)
	for _, toolCall := range assistantMessage.ToolCalls {
		spec, ok := execution.ToolByName[toolCall.Function.Name]
		if !ok {
			skillBinding, skillOK := execution.SkillByName[toolCall.Function.Name]
			if !skillOK {
				return nil, apperrors.New(apperrors.CodeInvalidArgument, "model returned unknown tool call")
			}
			s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeActionStarted, "Skill action started: "+skillBinding.Spec.Name+".", map[string]any{
				"action_id":   toolCall.ID,
				"type":        "skill",
				"target_id":   skillBinding.Spec.ID.String(),
				"target_name": skillBinding.Spec.Name,
				"input":       sanitizeValue(toolCall.Function.Arguments),
			})
			content, err := s.executeSkillToolCall(ctx, evt, skillBinding, toolCall, execution)
			if err != nil {
				s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeActionFailed, "Skill action failed: "+skillBinding.Spec.Name+".", map[string]any{
					"action_id":   toolCall.ID,
					"type":        "skill",
					"target_id":   skillBinding.Spec.ID.String(),
					"target_name": skillBinding.Spec.Name,
					"error":       err.Error(),
				})
				return nil, err
			}
			s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeActionCompleted, "Skill action completed: "+skillBinding.Spec.Name+".", map[string]any{
				"action_id":   toolCall.ID,
				"type":        "skill",
				"target_id":   skillBinding.Spec.ID.String(),
				"target_name": skillBinding.Spec.Name,
			})
			messages = append(messages, model.Message{
				Role:       model.RoleTool,
				Content:    content,
				ToolCallID: toolCall.ID,
			})
			continue
		}
		skillRefs := execution.SkillRefsByTool[spec.ID.String()]
		args := mergeToolCallArgs(defaultToolArgsForSpec(spec), execution.DefaultToolArgsByName[spec.Name])
		args = mergeToolCallArgs(args, toolCall.Function.Arguments)
		call := tool.Call{
			TenantID: evt.TenantID,
			ToolID:   spec.ID,
			Name:     spec.Name,
			Args:     args,
			TraceID:  evt.TraceID,
		}
		startedAt := time.Now()
		s.logger.Info("tool call started",
			"event_id", evt.ID.String(),
			"trace_id", evt.TraceID,
			"agent_id", evt.AgentID.String(),
			"tool_call_id", toolCall.ID,
			"tool_id", spec.ID.String(),
			"tool_name", spec.Name,
			"implementation", spec.Implementation,
			"arg_keys", argKeys(call.Args),
		)
		s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeActionStarted, "Tool action started: "+spec.Name+".", map[string]any{
			"action_id":      toolCall.ID,
			"type":           "tool",
			"target_id":      spec.ID.String(),
			"target_name":    spec.Name,
			"implementation": spec.Implementation,
			"arg_keys":       argKeys(call.Args),
			"input":          sanitizeValue(call.Args),
		})
		result, err := s.toolRuntime.ExecuteTool(ctx, call)
		if err != nil {
			s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeActionFailed, "Tool action failed: "+spec.Name+".", map[string]any{
				"action_id":   toolCall.ID,
				"type":        "tool",
				"target_id":   spec.ID.String(),
				"target_name": spec.Name,
				"error":       err.Error(),
			})
			s.appendTraceStep(
				ctx,
				evt,
				domain.TraceStepTool,
				spec.Name,
				domain.TraceStepStatusFailed,
				startedAt,
				time.Now().UTC(),
				toolFailureTraceMetadata(spec, call, skillRefs, err),
			)
			s.logger.Error("tool call failed",
				"event_id", evt.ID.String(),
				"trace_id", evt.TraceID,
				"agent_id", evt.AgentID.String(),
				"tool_call_id", toolCall.ID,
				"tool_id", spec.ID.String(),
				"tool_name", spec.Name,
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"error", err,
			)
			return nil, err
		}
		s.logger.Info("tool call completed",
			"event_id", evt.ID.String(),
			"trace_id", evt.TraceID,
			"agent_id", evt.AgentID.String(),
			"tool_call_id", toolCall.ID,
			"tool_id", spec.ID.String(),
			"tool_name", spec.Name,
			"success", result.Success,
			"error_code", result.ErrorCode,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
		s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeActionCompleted, "Tool action completed: "+spec.Name+".", map[string]any{
			"action_id":   toolCall.ID,
			"type":        "tool",
			"target_id":   spec.ID.String(),
			"target_name": spec.Name,
			"success":     result.Success,
			"error_code":  result.ErrorCode,
			"duration_ms": time.Since(startedAt).Milliseconds(),
		})
		finishedAt := time.Now().UTC()
		toolStatus := domain.TraceStepStatusSucceeded
		if !result.Success {
			toolStatus = domain.TraceStepStatusFailed
		}
		toolStep := s.appendTraceStep(
			ctx,
			evt,
			domain.TraceStepTool,
			spec.Name,
			toolStatus,
			startedAt,
			finishedAt,
			toolTraceMetadata(spec, call, result, skillRefs),
		)
		s.appendWorkflowTraceSteps(ctx, evt, toolStep.ID, spec, result)
		if !spec.Binding.ConnectorOperationID.Empty() {
			s.appendTraceStep(
				ctx,
				evt,
				domain.TraceStepConnector,
				spec.Binding.ConnectorOperationID.String(),
				toolStatus,
				startedAt,
				finishedAt,
				connectorTraceMetadata(spec, result),
			)
		}
		content, err := toolResultContent(result)
		if err != nil {
			return nil, err
		}
		messages = append(messages, model.Message{
			Role:       model.RoleTool,
			Content:    content,
			ToolCallID: toolCall.ID,
		})
	}

	return messages, nil
}

func argKeys(args map[string]any) []string {
	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	return keys
}

func mergeToolCallArgs(defaults map[string]any, args map[string]any) map[string]any {
	if len(defaults) == 0 {
		return args
	}
	merged := make(map[string]any, len(defaults)+len(args))
	for key, value := range defaults {
		merged[key] = value
	}
	for key, value := range args {
		if _, hasDefault := merged[key]; hasDefault && isEmptyToolArg(value) {
			continue
		}
		merged[key] = value
	}
	return merged
}

func isEmptyToolArg(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

// handleToolRequest executes an explicitly requested tool without involving a model.
//
// This path is intentionally kept separate from model-driven tool calling so
// deterministic workflow integrations and debugging requests do not depend on
// LLM behavior.
func (s *Service) handleToolRequest(ctx context.Context, evt event.Event, req domain.ToolRequest) (event.Response, error) {
	if s.toolRuntime == nil {
		return event.Response{}, apperrors.New(apperrors.CodeUnavailable, "tool runtime is not configured")
	}

	call := tool.Call{
		TenantID:  evt.TenantID,
		ToolID:    req.ToolID,
		Args:      req.Args,
		Confirmed: req.Confirmed,
		TraceID:   evt.TraceID,
	}

	startedAt := time.Now().UTC()
	result, err := s.toolRuntime.ExecuteTool(ctx, call)
	if err != nil {
		s.appendTraceStep(ctx, evt, domain.TraceStepTool, req.ToolID.String(), domain.TraceStepStatusFailed, startedAt, time.Now().UTC(), map[string]any{
			"tool_id":   req.ToolID.String(),
			"arg_keys":  argKeys(req.Args),
			"confirmed": req.Confirmed,
			"error":     err.Error(),
		})
		return event.Response{}, err
	}
	toolStep := s.appendTraceStep(ctx, evt, domain.TraceStepTool, req.ToolID.String(), domain.TraceStepStatusSucceeded, startedAt, time.Now().UTC(), map[string]any{
		"tool_id":    req.ToolID.String(),
		"arg_keys":   argKeys(req.Args),
		"confirmed":  req.Confirmed,
		"success":    result.Success,
		"error_code": result.ErrorCode,
	})
	s.appendWorkflowTraceSteps(ctx, evt, toolStep.ID, tool.Spec{ID: req.ToolID}, result)

	reply := "工具执行完成。"
	if !result.Success {
		reply = "工具执行失败。"
	}

	s.logger.Info("tool request handled",
		"event_id", evt.ID.String(),
		"trace_id", evt.TraceID,
		"tool_id", req.ToolID.String(),
		"success", result.Success,
	)

	return event.Response{
		EventID: evt.ID,
		TraceID: evt.TraceID,
		Actions: []event.Action{
			{
				Type:       event.ActionDisplayText,
				Text:       reply,
				ToolResult: &result,
			},
			{
				Type: event.ActionSpeak,
				Text: reply,
			},
			{
				Type: event.ActionEndTurn,
			},
		},
	}, nil
}

func normalizeEvent(ctx context.Context, evt event.Event) event.Event {
	if evt.ID.Empty() {
		evt.ID = id.New("evt")
	}
	if evt.TraceID == "" {
		_, traceID := trace.Ensure(ctx)
		evt.TraceID = traceID.String()
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}

	return evt
}

func textFromPayload(payload map[string]any) string {
	if payload == nil {
		return ""
	}

	value, _ := payload["text"].(string)
	return value
}

func runtimeSystemPromptFromPayload(payload map[string]any) string {
	if payload == nil {
		return ""
	}

	value, _ := payload[runtimeSystemPromptPayloadKey].(string)
	return strings.TrimSpace(value)
}

func runtimeDisableToolsFromPayload(payload map[string]any) bool {
	if payload == nil {
		return false
	}

	value, _ := payload[runtimeDisableToolsPayloadKey].(bool)
	return value
}

func appendRuntimeSystemPrompt(systemPrompt string, runtimePrompt string) string {
	runtimePrompt = strings.TrimSpace(runtimePrompt)
	if runtimePrompt == "" {
		return systemPrompt
	}
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" {
		return runtimePrompt
	}

	return systemPrompt + "\n\nRuntime Instructions:\n" + runtimePrompt
}

func (s *Service) loadAgentConfig(ctx context.Context, evt event.Event) (domain.AgentConfig, bool, error) {
	if evt.AgentID.Empty() || s.configLoader == nil {
		return domain.AgentConfig{}, false, nil
	}

	config, err := s.configLoader.LoadAgentConfig(ctx, evt.TenantID, evt.AgentID)
	if err != nil {
		return domain.AgentConfig{}, false, err
	}

	return config, true, nil
}

func toModelTools(specs []tool.Spec) []model.ToolDefinition {
	result := make([]model.ToolDefinition, 0, len(specs))
	functionNames := modelFunctionNameMapForTools(specs)
	for _, spec := range specs {
		parameters := spec.InputSchema
		if parameters == nil {
			parameters = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
		result = append(result, model.ToolDefinition{
			Type: "function",
			Function: model.ToolFunction{
				Name:        functionNames[spec.ID.String()],
				Description: modelToolDescription(spec),
				Parameters:  parameters,
			},
		})
	}
	return result
}

func modelToolDescription(spec tool.Spec) string {
	description := strings.TrimSpace(spec.LLMDescription)
	if description == "" {
		description = strings.TrimSpace(spec.Description)
	}
	if strings.TrimSpace(spec.Name) != "" {
		description = strings.TrimSpace(description + "\nTool display name: " + spec.Name)
	}
	if !spec.ID.Empty() {
		description = strings.TrimSpace(description + "\nTool ID: " + spec.ID.String())
	}
	if len(spec.OutputSchema) == 0 {
		return description
	}

	outputSchema, err := json.Marshal(spec.OutputSchema)
	if err != nil {
		return description
	}
	if description == "" {
		return "Returns JSON matching output_schema: " + string(outputSchema)
	}
	return description + "\nReturns JSON matching output_schema: " + string(outputSchema)
}

func mapToolsByName(specs []tool.Spec) map[string]tool.Spec {
	result := make(map[string]tool.Spec, len(specs))
	functionNames := modelFunctionNameMapForTools(specs)
	for _, spec := range specs {
		functionName := functionNames[spec.ID.String()]
		if functionName == "" {
			continue
		}
		result[functionName] = spec
	}
	return result
}

func toolChoice(tools []model.ToolDefinition) string {
	if len(tools) == 0 {
		return "none"
	}
	return "auto"
}

func toolResultContent(result tool.Result) (string, error) {
	bytes, err := json.Marshal(map[string]any{
		"success":      result.Success,
		"data":         result.Data,
		"error_code":   result.ErrorCode,
		"error_reason": result.ErrorReason,
	})
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeInternal, "failed to encode tool result", err)
	}
	return string(bytes), nil
}
