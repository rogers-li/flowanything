package application

import (
	"context"
	"encoding/json"
	"strings"

	"flow-anything/internal/contextengine"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/contracts/runtimeevent"
	"flow-anything/internal/platform/contracts/tool"
)

type promptToolExecution struct {
	SystemPrompt          string
	UserText              string
	History               []model.Message
	ModelName             string
	ModelOptions          model.Options
	Tools                 []model.ToolDefinition
	ToolByName            map[string]tool.Spec
	SkillByName           map[string]skillExecutionBinding
	SkillRefsByTool       map[string][]map[string]string
	DefaultToolArgsByName map[string]map[string]any
	MaxToolIterations     int
}

type promptToolRunResult struct {
	Messages            []model.Message
	Reply               string
	CurrentTurnMessages []model.Message
}

// executePromptToolRun is the shared execution loop for Agent and Skill.
//
// The caller decides which prompt and tools are available. The executor only
// owns the repeated model -> tool calls -> model cycle, so Skills can become
// executable without duplicating the Agent runtime implementation.
func (s *Service) executePromptToolRun(ctx context.Context, evt event.Event, run promptToolExecution) (promptToolRunResult, error) {
	maxToolIterations := run.MaxToolIterations
	if maxToolIterations <= 0 {
		maxToolIterations = s.options.MaxToolIterations
	}
	if maxToolIterations <= 0 {
		maxToolIterations = 1
	}

	assembly := contextengine.NewAssembler(s.options.ContextPolicy).Assemble(contextengine.Request{
		SystemPrompt: run.SystemPrompt,
		UserText:     run.UserText,
		History:      run.History,
	})
	messages := assembly.Messages
	currentTurnMessages := []model.Message{
		{Role: model.RoleUser, Content: run.UserText},
	}
	s.recordContextAssembly(ctx, evt, assembly.Report)

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		s.emitRuntimeEvent(ctx, evt, runtimeevent.TypePlanningStarted, "Planning next actions.", map[string]any{
			"iteration": iteration,
		})
		resp, err := s.chatModel(ctx, evt, "tool_iteration", iteration, model.ChatRequest{
			TenantID:   evt.TenantID,
			TraceID:    evt.TraceID,
			Model:      run.ModelName,
			Messages:   messages,
			Tools:      run.Tools,
			ToolChoice: toolChoice(run.Tools),
			Options:    run.ModelOptions,
		})
		if err != nil {
			return promptToolRunResult{}, err
		}
		if len(resp.Message.ToolCalls) == 0 {
			planningPayload, hasPlanningPayload := planningPayloadFromContent(resp.Message.Content)
			if !hasPlanningPayload {
				planningPayload = map[string]any{
					"iteration":                 iteration,
					"actions":                   []map[string]any{},
					"final_answer_if_no_action": resp.Message.Content,
				}
			} else {
				planningPayload["iteration"] = iteration
			}
			message := "No external action is needed."
			if hasPlanningPayload {
				message = "Planning result is ready."
			}
			s.emitRuntimeEvent(ctx, evt, runtimeevent.TypePlanningCompleted, message, planningPayload)
			for _, action := range plannedActionsFromPayload(planningPayload) {
				s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeActionPlanned, plannedActionMessage(action), map[string]any{
					"iteration": iteration,
					"action":    action,
				})
			}
			messages = append(messages, resp.Message)
			s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeAssistantMessageCompleted, "Assistant message completed.", map[string]any{
				"text": resp.Message.Content,
			})
			currentTurnMessages = append(currentTurnMessages, resp.Message)
			return promptToolRunResult{Messages: messages, Reply: resp.Message.Content, CurrentTurnMessages: currentTurnMessages}, nil
		}
		plannedActions := plannedActionsFromToolCalls(resp.Message.ToolCalls, run.ToolByName, run.SkillByName)
		s.emitRuntimeEvent(ctx, evt, runtimeevent.TypePlanningCompleted, "Planned external actions.", map[string]any{
			"iteration": iteration,
			"actions":   plannedActions,
		})
		for _, action := range plannedActions {
			s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeActionPlanned, plannedActionMessage(action), map[string]any{
				"iteration": iteration,
				"action":    action,
			})
		}

		beforeToolMessages := len(messages)
		messages, err = s.executeToolCalls(ctx, evt, messages, resp.Message, toolExecutionContext{
			ToolByName:            run.ToolByName,
			SkillByName:           run.SkillByName,
			SkillRefsByTool:       run.SkillRefsByTool,
			DefaultToolArgsByName: run.DefaultToolArgsByName,
			ModelName:             run.ModelName,
			ModelOptions:          run.ModelOptions,
		})
		if err != nil {
			return promptToolRunResult{}, err
		}
		currentTurnMessages = append(currentTurnMessages, messages[beforeToolMessages:]...)
		compacted := contextengine.NewAssembler(s.options.ContextPolicy).CompactMessages(messages)
		messages = compacted.Messages
		s.recordContextAssembly(ctx, evt, compacted.Report)
	}

	finalResp, err := s.chatModel(ctx, evt, "final_answer", maxToolIterations, model.ChatRequest{
		TenantID:   evt.TenantID,
		TraceID:    evt.TraceID,
		Model:      run.ModelName,
		Messages:   messages,
		ToolChoice: "none",
		Options:    run.ModelOptions,
	})
	if err != nil {
		return promptToolRunResult{}, err
	}
	messages = append(messages, finalResp.Message)
	currentTurnMessages = append(currentTurnMessages, finalResp.Message)
	s.emitRuntimeEvent(ctx, evt, runtimeevent.TypeAssistantMessageCompleted, "Assistant message completed.", map[string]any{
		"text": finalResp.Message.Content,
	})
	return promptToolRunResult{Messages: messages, Reply: finalResp.Message.Content, CurrentTurnMessages: currentTurnMessages}, nil
}

func plannedActionsFromToolCalls(calls []model.ToolCall, tools map[string]tool.Spec, skills map[string]skillExecutionBinding) []map[string]any {
	actions := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		action := map[string]any{
			"action_id": call.ID,
			"name":      call.Function.Name,
			"input":     call.Function.Arguments,
			"reason":    reasonFromArgs(call.Function.Arguments),
		}
		if spec, ok := tools[call.Function.Name]; ok {
			action["type"] = "tool"
			action["target_id"] = spec.ID.String()
			action["target_name"] = spec.Name
			action["implementation"] = spec.Implementation
		} else if binding, ok := skills[call.Function.Name]; ok {
			action["type"] = "skill"
			action["target_id"] = binding.Spec.ID.String()
			action["target_name"] = binding.Spec.Name
		} else {
			action["type"] = "tool"
		}
		actions = append(actions, action)
	}
	return actions
}

func reasonFromArgs(args map[string]any) string {
	for _, key := range []string{"reason", "why", "rationale"} {
		if value, ok := args[key].(string); ok && value != "" {
			return value
		}
	}
	return "The model selected this action for the current user request."
}

func plannedActionMessage(action map[string]any) string {
	actionType, _ := action["type"].(string)
	targetName, _ := action["target_name"].(string)
	if targetName == "" {
		targetName, _ = action["name"].(string)
	}
	if actionType == "" {
		actionType = "action"
	}
	if targetName == "" {
		return "Planned " + actionType + "."
	}
	return "Planned " + actionType + ": " + targetName + "."
}

func planningPayloadFromContent(content string) (map[string]any, bool) {
	cleaned := strings.TrimSpace(content)
	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return nil, false
	}
	if _, ok := payload["actions"]; ok {
		return payload, true
	}
	if _, ok := payload["next_node_ids"]; ok {
		return payload, true
	}
	if _, ok := payload["next_node_id"]; ok {
		return payload, true
	}
	return nil, false
}

func plannedActionsFromPayload(payload map[string]any) []map[string]any {
	rawActions, ok := payload["actions"].([]any)
	if ok {
		actions := make([]map[string]any, 0, len(rawActions))
		for _, raw := range rawActions {
			if action, ok := raw.(map[string]any); ok {
				actions = append(actions, action)
			}
		}
		return actions
	}
	nextNodeIDs, ok := payload["next_node_ids"].([]any)
	if !ok {
		if nextNodeID, ok := payload["next_node_id"].(string); ok && nextNodeID != "" {
			nextNodeIDs = []any{nextNodeID}
		}
	}
	actions := make([]map[string]any, 0, len(nextNodeIDs))
	reason, _ := payload["reason"].(string)
	for index, raw := range nextNodeIDs {
		nodeID, _ := raw.(string)
		if nodeID == "" {
			continue
		}
		actions = append(actions, map[string]any{
			"action_id":   nodeID,
			"type":        "agent",
			"target_id":   nodeID,
			"target_name": nodeID,
			"reason":      reason,
			"index":       index,
		})
	}
	return actions
}
