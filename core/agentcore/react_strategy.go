package agentcore

import (
	"fmt"

	"flow-anything/core/runtimecontext"
)

// ReActStrategy performs a bounded plan/act/observe loop. Each iteration can
// use observations from previous capability calls to decide whether to call
// more capabilities or produce a final answer.
type ReActStrategy struct{}

func (ReActStrategy) Name() string { return "react" }

func (ReActStrategy) Run(ctx Context, runtime StrategyRuntime, req AgentRunRequest) (AgentRunResult, error) {
	available := req.Agent.Capabilities
	if len(available) == 0 && runtime.Capabilities != nil {
		available = runtime.Capabilities.List()
	}
	policy := normalizeAgentPolicy(req.Agent.Policy)
	actionResults := make([]ActionResult, 0)

	for iteration := 1; iteration <= policy.MaxIterations; iteration++ {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningStarted, map[string]any{
			"iteration":    iteration,
			"capabilities": available,
			"observations": actionResults,
		}, ""))
		planningTrace := runtimecontext.TraceContext{
			TraceID:       req.TraceID,
			SpanID:        runtimecontext.AgentModelSpanID(req.TraceID, req.Agent.ID, fmt.Sprintf("react_planning_%d", iteration)),
			ParentSpanID:  req.TraceContext.SpanID,
			CorrelationID: req.TraceContext.CorrelationID,
		}
		planningMessages, err := assembleContextMessages(ctx, runtime, req, ReActStrategy{}.Name(), ContextPhaseReActPlanning, buildPlanningPrompt(req.Agent, available), actionResults, nil)
		if err != nil {
			return AgentRunResult{}, err
		}
		planningResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, planningTrace), ModelRequest{
			Model:        req.Agent.Model,
			Messages:     planningMessages,
			Tools:        available,
			Metadata:     map[string]any{"phase": "react_planning", "iteration": iteration},
			TraceID:      req.TraceID,
			TraceContext: planningTrace,
		})
		if err != nil {
			runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventPlanningFailed, map[string]any{"iteration": iteration}, err.Error()))
			return AgentRunResult{}, err
		}
		plan, err := parseActionPlan(planningResponse.Message.Content)
		if err != nil {
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
				return AgentRunResult{Text: plan.FinalAnswerIfNoAction, Output: output, Actions: actionResults, Raw: planningResponse.Raw}, nil
			}
			break
		}

		for _, action := range plan.Actions {
			if len(actionResults) >= policy.MaxActions {
				actionResults = append(actionResults, ActionResult{Action: action, Error: "max actions reached"})
				break
			}
			result := invokePlannedAction(ctx, runtime, req, action, iteration)
			actionResults = append(actionResults, result)
		}
		if len(actionResults) >= policy.MaxActions {
			break
		}
	}

	return finalizeFromObservations(ctx, runtime, req, actionResults)
}

func invokePlannedAction(ctx Context, runtime StrategyRuntime, req AgentRunRequest, action PlannedAction, iteration int) ActionResult {
	capability, ok := runtime.Capabilities.Get(action.ID)
	if !ok {
		errText := "capability not found"
		runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, ReActStrategy{}.Name(), EventCapabilityFailed, action, map[string]any{"iteration": iteration, "action": action}, errText))
		return ActionResult{Action: action, Error: errText}
	}
	descriptor := capability.Descriptor()
	if err := validateCapabilityInput(action, descriptor); err != nil {
		runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, ReActStrategy{}.Name(), EventCapabilityFailed, action, map[string]any{"iteration": iteration, "action": action}, err.Error()))
		return ActionResult{Action: action, Error: err.Error()}
	}
	runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, ReActStrategy{}.Name(), EventCapabilityStarted, action, map[string]any{"iteration": iteration, "action": action}, ""))
	capabilityTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentCapabilitySpanID(req.TraceID, req.Agent.ID, action.Type, fmt.Sprintf("%s@%d", action.ID, iteration)),
		ParentSpanID:  req.TraceContext.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
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
		runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, ReActStrategy{}.Name(), EventCapabilityFailed, action, map[string]any{"iteration": iteration, "action": action}, err.Error()))
		return ActionResult{Action: action, Error: err.Error()}
	}
	runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, ReActStrategy{}.Name(), EventCapabilityCompleted, action, map[string]any{"iteration": iteration, "action": action, "result": result}, ""))
	return ActionResult{Action: action, Result: result}
}

func finalizeFromObservations(ctx Context, runtime StrategyRuntime, req AgentRunRequest, actionResults []ActionResult) (AgentRunResult, error) {
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerStarted, nil, ""))
	finalAnswerTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentModelSpanID(req.TraceID, req.Agent.ID, "react_final_answer"),
		ParentSpanID:  req.TraceContext.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	finalMessages, err := assembleContextMessages(ctx, runtime, req, ReActStrategy{}.Name(), ContextPhaseFinalAnswer, buildFinalAnswerPrompt(req.Agent), actionResults, nil)
	if err != nil {
		return AgentRunResult{}, err
	}
	finalResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, finalAnswerTrace), ModelRequest{
		Model:        req.Agent.Model,
		Messages:     finalMessages,
		Metadata:     map[string]any{"phase": "react_final_answer"},
		TraceID:      req.TraceID,
		TraceContext: finalAnswerTrace,
	})
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerFailed, nil, err.Error()))
		return AgentRunResult{}, err
	}
	output, err := parseAndValidateFinalOutput(req.Agent, finalResponse.Message.Content)
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerFailed, map[string]any{"content": finalResponse.Message.Content}, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ReActStrategy{}.Name(), EventFinalAnswerCompleted, map[string]any{"content": finalResponse.Message.Content}, ""))
	return AgentRunResult{Text: finalResponse.Message.Content, Output: output, Actions: actionResults, Raw: finalResponse.Raw}, nil
}
