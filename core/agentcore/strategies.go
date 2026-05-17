package agentcore

import (
	corecapability "flow-anything/core/capability"
	coreprompt "flow-anything/core/prompt"
	"flow-anything/core/runtimecontext"
)

// DirectStrategy performs a single model call without external actions.
type DirectStrategy struct{}

func (DirectStrategy) Name() string { return "direct" }

func (DirectStrategy) Run(ctx Context, runtime StrategyRuntime, req AgentRunRequest) (AgentRunResult, error) {
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, DirectStrategy{}.Name(), EventModelStarted, map[string]any{"phase": "direct"}, ""))
	modelTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentModelSpanID(req.TraceID, req.Agent.ID, "direct"),
		ParentSpanID:  req.TraceContext.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	messages, err := assembleContextMessages(ctx, runtime, req, DirectStrategy{}.Name(), ContextPhaseDirect, req.Agent.Prompt, nil, nil)
	if err != nil {
		return AgentRunResult{}, err
	}
	response, err := runtime.Model.Chat(withAgentTraceContext(ctx, modelTrace), ModelRequest{
		Model:        req.Agent.Model,
		Messages:     messages,
		Metadata:     map[string]any{"phase": "direct"},
		TraceID:      req.TraceID,
		TraceContext: modelTrace,
	})
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, DirectStrategy{}.Name(), EventModelFailed, map[string]any{"phase": "direct"}, err.Error()))
		return AgentRunResult{}, err
	}
	output, err := parseAndValidateFinalOutput(req.Agent, response.Message.Content)
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, DirectStrategy{}.Name(), EventModelFailed, map[string]any{"phase": "direct", "content": response.Message.Content}, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, DirectStrategy{}.Name(), EventModelCompleted, map[string]any{"phase": "direct", "content": response.Message.Content}, ""))
	return AgentRunResult{Text: response.Message.Content, Output: output, Raw: response.Raw}, nil
}

// ActionPlanningStrategy asks the model for an action list, invokes selected
// capabilities, then asks the model to synthesize the final answer.
type ActionPlanningStrategy struct{}

func (ActionPlanningStrategy) Name() string { return "action-planning" }

func (ActionPlanningStrategy) Run(ctx Context, runtime StrategyRuntime, req AgentRunRequest) (AgentRunResult, error) {
	available := req.Agent.Capabilities
	if len(available) == 0 && runtime.Capabilities != nil {
		available = runtime.Capabilities.List()
	}

	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventPlanningStarted, map[string]any{"capabilities": available}, ""))
	planningTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentPlanningSpanID(req.TraceID, req.Agent.ID),
		ParentSpanID:  req.TraceContext.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	planningMessages, err := assembleContextMessages(ctx, runtime, req, ActionPlanningStrategy{}.Name(), ContextPhasePlanning, buildPlanningPrompt(req.Agent, available), nil, nil)
	if err != nil {
		return AgentRunResult{}, err
	}
	planningResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, planningTrace), ModelRequest{
		Model:        req.Agent.Model,
		Messages:     planningMessages,
		Tools:        available,
		Metadata:     map[string]any{"phase": "planning"},
		TraceID:      req.TraceID,
		TraceContext: planningTrace,
	})
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventPlanningFailed, nil, err.Error()))
		return AgentRunResult{}, err
	}

	plan, err := parseActionPlan(planningResponse.Message.Content)
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventPlanningFailed, map[string]any{"content": planningResponse.Message.Content}, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventPlanningCompleted, map[string]any{"actions": plan.Actions}, ""))

	if len(plan.Actions) == 0 {
		output, err := parseAndValidateFinalOutput(req.Agent, plan.FinalAnswerIfNoAction)
		if err != nil {
			runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventFinalAnswerFailed, map[string]any{"content": plan.FinalAnswerIfNoAction}, err.Error()))
			return AgentRunResult{}, err
		}
		return AgentRunResult{Text: plan.FinalAnswerIfNoAction, Output: output, Raw: planningResponse.Raw}, nil
	}

	actionResults := make([]ActionResult, 0, len(plan.Actions))
	policy := normalizeAgentPolicy(req.Agent.Policy)
	for _, action := range plan.Actions {
		if len(actionResults) >= policy.MaxActions {
			actionResults = append(actionResults, ActionResult{Action: action, Error: "max actions reached"})
			break
		}
		capability, ok := runtime.Capabilities.Get(action.ID)
		if !ok {
			actionResults = append(actionResults, ActionResult{Action: action, Error: "capability not found"})
			continue
		}
		descriptor := capability.Descriptor()
		if err := validateCapabilityInput(action, descriptor); err != nil {
			runtime.Events.PublishAgentEvent(ctx, capabilityEvent(req, EventCapabilityFailed, action, map[string]any{"action": action}, err.Error()))
			actionResults = append(actionResults, ActionResult{Action: action, Error: err.Error()})
			continue
		}
		runtime.Events.PublishAgentEvent(ctx, capabilityEvent(req, EventCapabilityStarted, action, map[string]any{"action": action}, ""))
		capabilityTrace := runtimecontext.TraceContext{
			TraceID:       req.TraceID,
			SpanID:        runtimecontext.AgentCapabilitySpanID(req.TraceID, req.Agent.ID, action.Type, action.ID),
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
			runtime.Events.PublishAgentEvent(ctx, capabilityEvent(req, EventCapabilityFailed, action, map[string]any{"action": action}, err.Error()))
			actionResults = append(actionResults, ActionResult{Action: action, Error: err.Error()})
			continue
		}
		runtime.Events.PublishAgentEvent(ctx, capabilityEvent(req, EventCapabilityCompleted, action, map[string]any{"action": action, "result": result}, ""))
		actionResults = append(actionResults, ActionResult{Action: action, Result: result})
	}

	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventFinalAnswerStarted, nil, ""))
	finalAnswerTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentFinalAnswerSpanID(req.TraceID, req.Agent.ID),
		ParentSpanID:  req.TraceContext.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	finalMessages, err := assembleContextMessages(ctx, runtime, req, ActionPlanningStrategy{}.Name(), ContextPhaseFinalAnswer, buildFinalAnswerPrompt(req.Agent), actionResults, nil)
	if err != nil {
		return AgentRunResult{}, err
	}
	finalResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, finalAnswerTrace), ModelRequest{
		Model:        req.Agent.Model,
		Messages:     finalMessages,
		Metadata:     map[string]any{"phase": "final_answer"},
		TraceID:      req.TraceID,
		TraceContext: finalAnswerTrace,
	})
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventFinalAnswerFailed, nil, err.Error()))
		return AgentRunResult{}, err
	}
	output, err := parseAndValidateFinalOutput(req.Agent, finalResponse.Message.Content)
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventFinalAnswerFailed, map[string]any{"content": finalResponse.Message.Content}, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventFinalAnswerCompleted, map[string]any{"content": finalResponse.Message.Content}, ""))
	return AgentRunResult{Text: finalResponse.Message.Content, Output: output, Actions: actionResults, Raw: finalResponse.Raw}, nil
}

type actionPlan struct {
	Actions               []PlannedAction `json:"actions"`
	FinalAnswerIfNoAction string          `json:"final_answer_if_no_action"`
}

func buildPlanningPrompt(agent AgentSpec, capabilities []CapabilityDescriptor) string {
	return coreprompt.BuildPlanningSystemPrompt(coreprompt.PlanningPromptRequest{
		AgentName:        agent.Name,
		AgentDescription: agent.Description,
		Base:             coreprompt.Spec{System: agent.Prompt},
		Capabilities:     toPromptCapabilities(capabilities),
		OutputSchema:     agent.OutputSchema,
	})
}

func buildFinalAnswerPrompt(agent AgentSpec) string {
	return coreprompt.BuildFinalAnswerSystemPrompt(coreprompt.Spec{System: agent.Prompt}, agent.OutputSchema)
}

func parseActionPlan(content string) (actionPlan, error) {
	parsed, err := corecapability.ParseActionPlan(content)
	if err != nil {
		return actionPlan{}, err
	}
	plan := actionPlan{
		FinalAnswerIfNoAction: parsed.FinalAnswerIfNoAction,
		Actions:               make([]PlannedAction, 0, len(parsed.Actions)),
	}
	for _, action := range parsed.Actions {
		plan.Actions = append(plan.Actions, PlannedAction{
			Type:   string(action.Kind),
			ID:     action.ID,
			Task:   action.Task,
			Input:  action.Input,
			Reason: action.Reason,
		})
	}
	return plan, nil
}

func toPromptCapabilities(capabilities []CapabilityDescriptor) []coreprompt.CapabilityDescriptor {
	out := make([]coreprompt.CapabilityDescriptor, 0, len(capabilities))
	for _, capability := range capabilities {
		out = append(out, coreprompt.CapabilityDescriptor{
			ID:           capability.ID,
			Kind:         capability.Type,
			Name:         capability.Name,
			Description:  capability.Description,
			InputSchema:  capability.InputSchema,
			OutputSchema: capability.OutputSchema,
		})
	}
	return out
}

func strategyEvent(req AgentRunRequest, strategy string, eventType AgentEventType, data map[string]any, errText string) AgentEvent {
	event := newAgentEvent(req, eventType, data, errText)
	event.Strategy = strategy
	return event
}

func capabilityEvent(req AgentRunRequest, eventType AgentEventType, action PlannedAction, data map[string]any, errText string) AgentEvent {
	return capabilityEventForStrategy(req, ActionPlanningStrategy{}.Name(), eventType, action, data, errText)
}

func capabilityEventForStrategy(req AgentRunRequest, strategy string, eventType AgentEventType, action PlannedAction, data map[string]any, errText string) AgentEvent {
	event := newAgentEvent(req, eventType, data, errText)
	event.Strategy = strategy
	event.CapabilityID = action.ID
	event.CapabilityType = action.Type
	return event
}
