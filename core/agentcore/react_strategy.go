package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"

	"flow-anything/core/runtimecontext"
)

// ReActStrategy performs a bounded plan/act/observe loop. Each iteration can
// use observations from previous capability calls to decide whether to call
// more capabilities or produce a final answer.
type ReActStrategy struct{}

func (ReActStrategy) Name() string { return "react" }

func (ReActStrategy) Run(ctx Context, runtime StrategyRuntime, req AgentRunRequest) (AgentRunResult, error) {
	available := req.Agent.Capabilities
	policy := normalizeAgentPolicy(req.Agent.Policy)
	actionResults := make([]ActionResult, 0)
	attemptedActionFingerprints := map[string]struct{}{}

	for iteration := 1; iteration <= policy.MaxIterations; iteration++ {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningStarted, map[string]any{
			"iteration":    iteration,
			"capabilities": available,
			"observations": actionResults,
		}, ""))
		planningSpanID := runtimecontext.AgentPlanningSpanID(req.TraceID, req.Agent.ID)
		planningTrace := runtimecontext.TraceContext{
			TraceID:       req.TraceID,
			SpanID:        runtimecontext.AgentModelSpanID(req.TraceID, req.Agent.ID, fmt.Sprintf("react_planning_%d", iteration)),
			ParentSpanID:  planningSpanID,
			CorrelationID: req.TraceContext.CorrelationID,
		}
		planningMessages, err := assembleContextMessages(ctx, runtime, req, ReActStrategy{}.Name(), ContextPhaseReActPlanning, buildPlanningPrompt(req.Agent, available), actionResults, nil)
		if err != nil {
			return AgentRunResult{}, err
		}
		publishModelStarted(ctx, runtime, req, ReActStrategy{}.Name(), planningTrace, fmt.Sprintf("react_planning_%d", iteration), planningMessages, available)
		planningResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, planningTrace), ModelRequest{
			Model:        req.Agent.Model,
			Messages:     planningMessages,
			Tools:        available,
			Metadata:     map[string]any{"phase": "react_planning", "iteration": iteration},
			TraceID:      req.TraceID,
			TraceContext: planningTrace,
		})
		if err != nil {
			publishModelFailed(ctx, runtime, req, ReActStrategy{}.Name(), planningTrace, fmt.Sprintf("react_planning_%d", iteration), err.Error())
			runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningFailed, map[string]any{"iteration": iteration}, err.Error()))
			return AgentRunResult{}, err
		}
		publishModelCompleted(ctx, runtime, req, ReActStrategy{}.Name(), planningTrace, fmt.Sprintf("react_planning_%d", iteration), planningResponse)
		plan, err := parseActionPlan(planningResponse.Message.Content)
		if err != nil {
			if len(actionResults) > 0 && strings.TrimSpace(planningResponse.Message.Content) != "" {
				if looksLikeMalformedActionPlan(planningResponse.Message.Content) {
					runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningCompleted, map[string]any{
						"iteration": iteration,
						"actions":   []PlannedAction{},
						"content":   planningResponse.Message.Content,
						"fallback":  "final_answer_from_observations",
					}, ""))
					return finalizeFromObservations(ctx, runtime, req, actionResults)
				}
				output, finalErr := parseAndValidateFinalOutput(req.Agent, planningResponse.Message.Content)
				if finalErr != nil {
					runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerFailed, map[string]any{"iteration": iteration, "content": planningResponse.Message.Content}, finalErr.Error()))
					return AgentRunResult{}, finalErr
				}
				runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningCompleted, map[string]any{
					"iteration": iteration,
					"actions":   []PlannedAction{},
					"content":   planningResponse.Message.Content,
					"fallback":  "natural_final_answer",
				}, ""))
				runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerCompleted, map[string]any{
					"iteration": iteration,
					"content":   planningResponse.Message.Content,
					"source":    "react_planning_response",
				}, ""))
				return AgentRunResult{Text: finalTextFromOutput(planningResponse.Message.Content, output), Output: output, Actions: actionResults, Raw: planningResponse.Raw}, nil
			}
			runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningFailed, map[string]any{"iteration": iteration, "content": planningResponse.Message.Content}, err.Error()))
			return AgentRunResult{}, err
		}
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningCompleted, map[string]any{"iteration": iteration, "actions": plan.Actions}, ""))

		if len(plan.Actions) == 0 {
			if plan.FinalAnswerIfNoAction != "" {
				output, err := parseAndValidateFinalOutput(req.Agent, plan.FinalAnswerIfNoAction)
				if err != nil {
					runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerFailed, map[string]any{"content": plan.FinalAnswerIfNoAction}, err.Error()))
					return AgentRunResult{}, err
				}
				return AgentRunResult{Text: finalTextFromOutput(plan.FinalAnswerIfNoAction, output), Output: output, Actions: actionResults, Raw: planningResponse.Raw}, nil
			}
			break
		}

		resolvedActions, unavailableActions := resolvePlannedActions(runtime.Capabilities, available, plan.Actions)
		if len(unavailableActions) > 0 {
			runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningCompleted, map[string]any{
				"iteration":                   iteration,
				"actions":                     resolvedActions,
				"skipped_unavailable_actions": unavailableActions,
			}, ""))
		}
		if len(resolvedActions) == 0 {
			break
		}

		nextActions, duplicateActions := filterRepeatedActions(resolvedActions, attemptedActionFingerprints)
		if len(duplicateActions) > 0 {
			runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningCompleted, map[string]any{
				"iteration":                 iteration,
				"actions":                   nextActions,
				"skipped_duplicate_actions": duplicateActions,
				"loop_breaker":              len(nextActions) == 0,
			}, ""))
		}
		if len(nextActions) == 0 {
			break
		}

		for _, action := range nextActions {
			if len(actionResults) >= policy.MaxActions {
				actionResults = append(actionResults, ActionResult{Action: action, Error: "max actions reached"})
				break
			}
			attemptedActionFingerprints[plannedActionFingerprint(action)] = struct{}{}
			result := invokePlannedAction(ctx, runtime, req, action, iteration)
			actionResults = append(actionResults, result)
		}
		if len(actionResults) >= policy.MaxActions {
			break
		}
	}

	return finalizeFromObservations(ctx, runtime, req, actionResults)
}

func filterRepeatedActions(actions []PlannedAction, attempted map[string]struct{}) ([]PlannedAction, []PlannedAction) {
	nextActions := make([]PlannedAction, 0, len(actions))
	duplicateActions := make([]PlannedAction, 0)
	seenInPlan := map[string]struct{}{}
	for _, action := range actions {
		fingerprint := plannedActionFingerprint(action)
		if _, seen := attempted[fingerprint]; seen {
			duplicateActions = append(duplicateActions, action)
			continue
		}
		if _, seen := seenInPlan[fingerprint]; seen {
			duplicateActions = append(duplicateActions, action)
			continue
		}
		seenInPlan[fingerprint] = struct{}{}
		nextActions = append(nextActions, action)
	}
	return nextActions, duplicateActions
}

func plannedActionFingerprint(action PlannedAction) string {
	actionType := strings.ToLower(action.Type)
	if actionType == "agent" || actionType == "skill" || actionType == "workflow" {
		return fmt.Sprintf("%s|%s", actionType, action.ID)
	}
	inputBytes, err := json.Marshal(action.Input)
	if err != nil {
		inputBytes = []byte(fmt.Sprintf("%v", action.Input))
	}
	if actionType == "tool" || actionType == "connector" {
		return fmt.Sprintf("%s|%s|%s", actionType, action.ID, string(inputBytes))
	}
	return fmt.Sprintf("%s|%s|%s|%s", actionType, action.ID, action.Task, string(inputBytes))
}

func invokePlannedAction(ctx Context, runtime StrategyRuntime, req AgentRunRequest, action PlannedAction, iteration int) ActionResult {
	capabilityTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentCapabilitySpanID(req.TraceID, req.Agent.ID, action.Type, fmt.Sprintf("%s@%d", action.ID, iteration)),
		ParentSpanID:  req.TraceContext.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	capability, ok := runtime.Capabilities.Get(action.ID)
	if !ok {
		errText := "capability not found"
		runtime.Events.PublishAgentEvent(ctx, capabilityEventWithTraceForStrategy(req, ReActStrategy{}.Name(), EventCapabilityFailed, action, capabilityTrace, map[string]any{"iteration": iteration, "action": action}, errText))
		return ActionResult{Action: action, Error: errText}
	}
	descriptor := capability.Descriptor()
	if err := validateCapabilityInput(action, descriptor); err != nil {
		runtime.Events.PublishAgentEvent(ctx, capabilityEventWithTraceForStrategy(req, ReActStrategy{}.Name(), EventCapabilityFailed, action, capabilityTrace, map[string]any{"iteration": iteration, "action": action}, err.Error()))
		return ActionResult{Action: action, Error: err.Error()}
	}
	runtime.Events.PublishAgentEvent(ctx, capabilityEventWithTraceForStrategy(req, ReActStrategy{}.Name(), EventCapabilityStarted, action, capabilityTrace, map[string]any{"iteration": iteration, "action": action}, ""))
	result, err := capability.Invoke(withAgentTraceContext(ctx, capabilityTrace), CapabilityCall{
		ID:           action.ID,
		Type:         action.Type,
		Task:         action.Task,
		Input:        action.Input,
		Reason:       action.Reason,
		TraceID:      req.TraceID,
		TraceContext: capabilityTrace,
	})
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, capabilityEventWithTraceForStrategy(req, ReActStrategy{}.Name(), EventCapabilityFailed, action, capabilityTrace, map[string]any{"iteration": iteration, "action": action}, err.Error()))
		return ActionResult{Action: action, Error: err.Error()}
	}
	runtime.Events.PublishAgentEvent(ctx, capabilityEventWithTraceForStrategy(req, ReActStrategy{}.Name(), EventCapabilityCompleted, action, capabilityTrace, map[string]any{"iteration": iteration, "action": action, "result": result}, ""))
	return ActionResult{Action: action, Result: result}
}

func finalizeFromObservations(ctx Context, runtime StrategyRuntime, req AgentRunRequest, actionResults []ActionResult) (AgentRunResult, error) {
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerStarted, nil, ""))
	finalAnswerTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentModelSpanID(req.TraceID, req.Agent.ID, "react_final_answer"),
		ParentSpanID:  runtimecontext.AgentFinalAnswerSpanID(req.TraceID, req.Agent.ID),
		CorrelationID: req.TraceContext.CorrelationID,
	}
	finalMessages, err := assembleContextMessages(ctx, runtime, req, ReActStrategy{}.Name(), ContextPhaseFinalAnswer, buildFinalAnswerPrompt(req.Agent), actionResults, nil)
	if err != nil {
		return AgentRunResult{}, err
	}
	publishModelStarted(ctx, runtime, req, ReActStrategy{}.Name(), finalAnswerTrace, "react_final_answer", finalMessages, nil)
	finalResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, finalAnswerTrace), ModelRequest{
		Model:        req.Agent.Model,
		Messages:     finalMessages,
		Metadata:     map[string]any{"phase": "react_final_answer"},
		TraceID:      req.TraceID,
		TraceContext: finalAnswerTrace,
	})
	if err != nil {
		publishModelFailed(ctx, runtime, req, ReActStrategy{}.Name(), finalAnswerTrace, "react_final_answer", err.Error())
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerFailed, nil, err.Error()))
		return AgentRunResult{}, err
	}
	publishModelCompleted(ctx, runtime, req, ReActStrategy{}.Name(), finalAnswerTrace, "react_final_answer", finalResponse)
	output, err := parseAndValidateFinalOutput(req.Agent, finalResponse.Message.Content)
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerFailed, map[string]any{"content": finalResponse.Message.Content}, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerCompleted, map[string]any{"content": finalResponse.Message.Content}, ""))
	return AgentRunResult{Text: finalTextFromOutput(finalResponse.Message.Content, output), Output: output, Actions: actionResults, Raw: finalResponse.Raw}, nil
}

func looksLikeMalformedActionPlan(content string) bool {
	normalized := strings.ToLower(content)
	return strings.Contains(normalized, `"actions"`) || strings.Contains(normalized, `"type"`) && strings.Contains(normalized, `"input"`)
}
