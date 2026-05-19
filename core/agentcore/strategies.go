package agentcore

import (
	"strings"

	corecapability "flow-anything/core/capability"
	coreprompt "flow-anything/core/prompt"
	"flow-anything/core/runtimecontext"
)

// DirectStrategy performs a single model call without external actions.
type DirectStrategy struct{}

func (DirectStrategy) Name() string { return "direct" }

func (DirectStrategy) Run(ctx Context, runtime StrategyRuntime, req AgentRunRequest) (AgentRunResult, error) {
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
	publishModelStarted(ctx, runtime, req, DirectStrategy{}.Name(), modelTrace, "direct", messages, nil)
	response, err := runtime.Model.Chat(withAgentTraceContext(ctx, modelTrace), ModelRequest{
		Model:        req.Agent.Model,
		Messages:     messages,
		Metadata:     map[string]any{"phase": "direct"},
		TraceID:      req.TraceID,
		TraceContext: modelTrace,
	})
	if err != nil {
		publishModelFailed(ctx, runtime, req, DirectStrategy{}.Name(), modelTrace, "direct", err.Error())
		return AgentRunResult{}, err
	}
	output, err := parseAndValidateFinalOutput(req.Agent, response.Message.Content)
	if err != nil {
		publishModelFailed(ctx, runtime, req, DirectStrategy{}.Name(), modelTrace, "direct", err.Error(), response)
		return AgentRunResult{}, err
	}
	publishModelCompleted(ctx, runtime, req, DirectStrategy{}.Name(), modelTrace, "direct", response)
	return AgentRunResult{Text: finalTextFromOutput(response.Message.Content, output), Output: output, Raw: response.Raw}, nil
}

// ActionPlanningStrategy asks the model for an action list, invokes selected
// capabilities, then asks the model to synthesize the final answer.
type ActionPlanningStrategy struct{}

func (ActionPlanningStrategy) Name() string { return "action-planning" }

func (ActionPlanningStrategy) Run(ctx Context, runtime StrategyRuntime, req AgentRunRequest) (AgentRunResult, error) {
	return runPlanExecuteSolveStrategy(ctx, runtime, req, ActionPlanningStrategy{}.Name())
}

// ReWOOStrategy implements Reasoning Without Observation. It asks the model to
// plan all required actions up front, executes the selected capabilities once,
// then asks the model to solve from those results. Unlike ReAct, observations
// are not fed into another planning loop, which keeps recursive Agent Graph
// execution bounded and predictable.
type ReWOOStrategy struct{}

func (ReWOOStrategy) Name() string { return "rewoo" }

func (ReWOOStrategy) Run(ctx Context, runtime StrategyRuntime, req AgentRunRequest) (AgentRunResult, error) {
	return runPlanExecuteSolveStrategy(ctx, runtime, req, ReWOOStrategy{}.Name())
}

func runPlanExecuteSolveStrategy(ctx Context, runtime StrategyRuntime, req AgentRunRequest, strategyName string) (AgentRunResult, error) {
	available := req.Agent.Capabilities

	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventPlanningStarted, map[string]any{"capabilities": available}, ""))
	planningSpanID := runtimecontext.AgentPlanningSpanID(req.TraceID, req.Agent.ID)
	planningMessages, err := assembleContextMessages(ctx, runtime, req, strategyName, ContextPhasePlanning, buildPlanningPrompt(req.Agent, available), nil, nil)
	if err != nil {
		return AgentRunResult{}, err
	}
	planningModelTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentModelSpanID(req.TraceID, req.Agent.ID, "planning"),
		ParentSpanID:  planningSpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	publishModelStarted(ctx, runtime, req, strategyName, planningModelTrace, "planning", planningMessages, available)
	planningResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, planningModelTrace), ModelRequest{
		Model:        req.Agent.Model,
		Messages:     planningMessages,
		Tools:        available,
		Metadata:     map[string]any{"phase": "planning"},
		TraceID:      req.TraceID,
		TraceContext: planningModelTrace,
	})
	if err != nil {
		publishModelFailed(ctx, runtime, req, strategyName, planningModelTrace, "planning", err.Error())
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventPlanningFailed, nil, err.Error()))
		return AgentRunResult{}, err
	}
	publishModelCompleted(ctx, runtime, req, strategyName, planningModelTrace, "planning", planningResponse)

	plan, err := parseActionPlan(planningResponse.Message.Content)
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventPlanningFailed, map[string]any{"content": planningResponse.Message.Content}, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventPlanningCompleted, map[string]any{"actions": plan.Actions}, ""))

	if len(plan.Actions) == 0 {
		output, err := parseAndValidateFinalOutput(req.Agent, plan.FinalAnswerIfNoAction)
		if err != nil {
			runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventFinalAnswerFailed, map[string]any{"content": plan.FinalAnswerIfNoAction}, err.Error()))
			return AgentRunResult{}, err
		}
		return AgentRunResult{Text: finalTextFromOutput(plan.FinalAnswerIfNoAction, output), Output: output, Raw: planningResponse.Raw}, nil
	}

	actionResults := make([]ActionResult, 0, len(plan.Actions))
	policy := normalizeAgentPolicy(req.Agent.Policy)
	for _, action := range plan.Actions {
		if len(actionResults) >= policy.MaxActions {
			actionResults = append(actionResults, ActionResult{Action: action, Error: "max actions reached"})
			break
		}
		capability, resolvedAction, ok := resolvePlannedCapability(runtime.Capabilities, available, action)
		if !ok {
			actionResults = append(actionResults, ActionResult{Action: action, Error: "capability not found"})
			continue
		}
		action = resolvedAction
		descriptor := capability.Descriptor()
		if err := validateCapabilityInput(action, descriptor); err != nil {
			runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, strategyName, EventCapabilityFailed, action, map[string]any{"action": action}, err.Error()))
			actionResults = append(actionResults, ActionResult{Action: action, Error: err.Error()})
			continue
		}
		runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, strategyName, EventCapabilityStarted, action, map[string]any{"action": action}, ""))
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
			runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, strategyName, EventCapabilityFailed, action, map[string]any{"action": action}, err.Error()))
			actionResults = append(actionResults, ActionResult{Action: action, Error: err.Error()})
			continue
		}
		runtime.Events.PublishAgentEvent(ctx, capabilityEventForStrategy(req, strategyName, EventCapabilityCompleted, action, map[string]any{"action": action, "result": result}, ""))
		actionResults = append(actionResults, ActionResult{Action: action, Result: result})
	}

	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventFinalAnswerStarted, nil, ""))
	finalAnswerTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentFinalAnswerSpanID(req.TraceID, req.Agent.ID),
		ParentSpanID:  req.TraceContext.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	finalMessages, err := assembleContextMessages(ctx, runtime, req, strategyName, ContextPhaseFinalAnswer, buildFinalAnswerPrompt(req.Agent), actionResults, nil)
	if err != nil {
		return AgentRunResult{}, err
	}
	finalModelTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentModelSpanID(req.TraceID, req.Agent.ID, "final_answer"),
		ParentSpanID:  finalAnswerTrace.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	publishModelStarted(ctx, runtime, req, strategyName, finalModelTrace, "final_answer", finalMessages, nil)
	finalResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, finalModelTrace), ModelRequest{
		Model:        req.Agent.Model,
		Messages:     finalMessages,
		Metadata:     map[string]any{"phase": "final_answer"},
		TraceID:      req.TraceID,
		TraceContext: finalModelTrace,
	})
	if err != nil {
		publishModelFailed(ctx, runtime, req, strategyName, finalModelTrace, "final_answer", err.Error())
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventFinalAnswerFailed, nil, err.Error()))
		return AgentRunResult{}, err
	}
	publishModelCompleted(ctx, runtime, req, strategyName, finalModelTrace, "final_answer", finalResponse)
	output, err := parseAndValidateFinalOutput(req.Agent, finalResponse.Message.Content)
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventFinalAnswerFailed, map[string]any{"content": finalResponse.Message.Content}, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategyName, EventFinalAnswerCompleted, map[string]any{"content": finalResponse.Message.Content}, ""))
	return AgentRunResult{Text: finalTextFromOutput(finalResponse.Message.Content, output), Output: output, Actions: actionResults, Raw: finalResponse.Raw}, nil
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

func publishModelStarted(ctx Context, runtime StrategyRuntime, req AgentRunRequest, strategy string, trace runtimecontext.TraceContext, phase string, messages []Message, tools []CapabilityDescriptor) {
	runtime.Events.PublishAgentEvent(ctx, AgentEvent{
		Type:         EventModelStarted,
		TraceID:      req.TraceID,
		TraceContext: trace,
		AgentID:      req.Agent.ID,
		Strategy:     strategy,
		Data: map[string]any{
			"phase": phase,
			"request": map[string]any{
				"model":         req.Agent.Model,
				"messages":      messages,
				"tools":         tools,
				"tool_count":    len(tools),
				"message_count": len(messages),
			},
		},
	})
}

func publishModelCompleted(ctx Context, runtime StrategyRuntime, req AgentRunRequest, strategy string, trace runtimecontext.TraceContext, phase string, response ModelResponse) {
	runtime.Events.PublishAgentEvent(ctx, AgentEvent{
		Type:         EventModelCompleted,
		TraceID:      req.TraceID,
		TraceContext: trace,
		AgentID:      req.Agent.ID,
		Strategy:     strategy,
		Data: map[string]any{
			"phase": phase,
			"response": map[string]any{
				"message":  response.Message,
				"raw":      response.Raw,
				"usage":    response.Usage,
				"provider": response.Provider,
				"model":    response.Model,
			},
		},
	})
}

func publishModelFailed(ctx Context, runtime StrategyRuntime, req AgentRunRequest, strategy string, trace runtimecontext.TraceContext, phase string, errText string, response ...ModelResponse) {
	data := map[string]any{"phase": phase}
	if len(response) > 0 {
		data["response"] = map[string]any{
			"message":  response[0].Message,
			"raw":      response[0].Raw,
			"usage":    response[0].Usage,
			"provider": response[0].Provider,
			"model":    response[0].Model,
		}
	}
	runtime.Events.PublishAgentEvent(ctx, AgentEvent{
		Type:         EventModelFailed,
		TraceID:      req.TraceID,
		TraceContext: trace,
		AgentID:      req.Agent.ID,
		Strategy:     strategy,
		Data:         data,
		Error:        errText,
	})
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

func resolvePlannedActions(registry CapabilityRegistry, available []CapabilityDescriptor, actions []PlannedAction) ([]PlannedAction, []PlannedAction) {
	resolved := make([]PlannedAction, 0, len(actions))
	unavailable := make([]PlannedAction, 0)
	for _, action := range actions {
		if _, resolvedAction, ok := resolvePlannedCapability(registry, available, action); ok {
			resolved = append(resolved, resolvedAction)
			continue
		}
		unavailable = append(unavailable, action)
	}
	return resolved, unavailable
}

func resolvePlannedCapability(registry CapabilityRegistry, available []CapabilityDescriptor, action PlannedAction) (Capability, PlannedAction, bool) {
	if registry == nil {
		return nil, action, false
	}
	descriptor, ok := matchAvailableCapability(available, action.ID)
	if !ok {
		return nil, action, false
	}
	capability, ok := registry.Get(descriptor.ID)
	if !ok {
		return nil, action, false
	}
	action.ID = descriptor.ID
	action.Type = descriptor.Type
	return capability, action, true
}

func matchAvailableCapability(available []CapabilityDescriptor, plannedID string) (CapabilityDescriptor, bool) {
	plannedID = strings.TrimSpace(plannedID)
	if plannedID == "" {
		return CapabilityDescriptor{}, false
	}
	for _, descriptor := range available {
		if descriptor.ID == plannedID {
			return descriptor, true
		}
	}
	for _, descriptor := range available {
		if descriptor.Name == plannedID {
			return descriptor, true
		}
	}
	for _, descriptor := range available {
		if strings.EqualFold(descriptor.ID, plannedID) || strings.EqualFold(descriptor.Name, plannedID) {
			return descriptor, true
		}
	}
	return CapabilityDescriptor{}, false
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

func capabilityEventWithTraceForStrategy(req AgentRunRequest, strategy string, eventType AgentEventType, action PlannedAction, trace runtimecontext.TraceContext, data map[string]any, errText string) AgentEvent {
	event := capabilityEventForStrategy(req, strategy, eventType, action, data, errText)
	event.TraceContext = trace
	return event
}
