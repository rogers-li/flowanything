package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"

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
	response, err := runtime.Model.Chat(withAgentTraceContext(ctx, modelTrace), ModelRequest{
		Model: req.Agent.Model,
		Messages: append([]Message{{Role: "system", Content: req.Agent.Prompt}},
			append(req.Conversation, Message{Role: "user", Content: req.UserMessage})...,
		),
		Metadata:     map[string]any{"phase": "direct"},
		TraceID:      req.TraceID,
		TraceContext: modelTrace,
	})
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, DirectStrategy{}.Name(), EventModelFailed, map[string]any{"phase": "direct"}, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, DirectStrategy{}.Name(), EventModelCompleted, map[string]any{"phase": "direct", "content": response.Message.Content}, ""))
	return AgentRunResult{Text: response.Message.Content, Raw: response.Raw}, nil
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
	planningResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, planningTrace), ModelRequest{
		Model: req.Agent.Model,
		Messages: []Message{
			{Role: "system", Content: buildPlanningPrompt(req.Agent, available)},
			{Role: "user", Content: req.UserMessage},
		},
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
		return AgentRunResult{Text: plan.FinalAnswerIfNoAction, Raw: planningResponse.Raw}, nil
	}

	actionResults := make([]ActionResult, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		capability, ok := runtime.Capabilities.Get(action.ID)
		if !ok {
			actionResults = append(actionResults, ActionResult{Action: action, Error: "capability not found"})
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

	observationBytes, _ := json.MarshalIndent(actionResults, "", "  ")
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventFinalAnswerStarted, nil, ""))
	finalAnswerTrace := runtimecontext.TraceContext{
		TraceID:       req.TraceID,
		SpanID:        runtimecontext.AgentFinalAnswerSpanID(req.TraceID, req.Agent.ID),
		ParentSpanID:  req.TraceContext.SpanID,
		CorrelationID: req.TraceContext.CorrelationID,
	}
	finalResponse, err := runtime.Model.Chat(withAgentTraceContext(ctx, finalAnswerTrace), ModelRequest{
		Model: req.Agent.Model,
		Messages: []Message{
			{Role: "system", Content: req.Agent.Prompt + "\n\nUse the observations to answer the user naturally."},
			{Role: "user", Content: req.UserMessage},
			{Role: "assistant", Content: "Observations:\n" + string(observationBytes)},
		},
		Metadata:     map[string]any{"phase": "final_answer"},
		TraceID:      req.TraceID,
		TraceContext: finalAnswerTrace,
	})
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventFinalAnswerFailed, nil, err.Error()))
		return AgentRunResult{}, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, ActionPlanningStrategy{}.Name(), EventFinalAnswerCompleted, map[string]any{"content": finalResponse.Message.Content}, ""))
	return AgentRunResult{Text: finalResponse.Message.Content, Actions: actionResults, Raw: finalResponse.Raw}, nil
}

type actionPlan struct {
	Actions               []PlannedAction `json:"actions"`
	FinalAnswerIfNoAction string          `json:"final_answer_if_no_action"`
}

func buildPlanningPrompt(agent AgentSpec, capabilities []CapabilityDescriptor) string {
	var builder strings.Builder
	builder.WriteString(agent.Prompt)
	builder.WriteString("\n\nRuntime Action Planning Contract:\n")
	builder.WriteString("Plan which capabilities to invoke. Return JSON only, with no markdown.\n")
	builder.WriteString(`Schema: {"actions":[{"type":"tool|skill|agent|workflow","id":"...","task":"specific task","input":{},"reason":"why this action is needed"}],"final_answer_if_no_action":"optional answer"}`)
	builder.WriteString("\nConstraints:\n")
	builder.WriteString("- Select only from the available capabilities below.\n")
	builder.WriteString("- Keep each action task self-contained.\n")
	builder.WriteString("- If no capability is needed, return an empty actions array and final_answer_if_no_action.\n")
	builder.WriteString("\nAvailable capabilities:\n")
	for _, capability := range capabilities {
		builder.WriteString(fmt.Sprintf("- type=%s; id=%s; name=%s; description=%s\n", capability.Type, capability.ID, capability.Name, capability.Description))
	}
	return builder.String()
}

func parseActionPlan(content string) (actionPlan, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var plan actionPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return actionPlan{}, err
	}
	return plan, nil
}

func strategyEvent(req AgentRunRequest, strategy string, eventType AgentEventType, data map[string]any, errText string) AgentEvent {
	event := newAgentEvent(req, eventType, data, errText)
	event.Strategy = strategy
	return event
}

func capabilityEvent(req AgentRunRequest, eventType AgentEventType, action PlannedAction, data map[string]any, errText string) AgentEvent {
	event := newAgentEvent(req, eventType, data, errText)
	event.Strategy = ActionPlanningStrategy{}.Name()
	event.CapabilityID = action.ID
	event.CapabilityType = action.Type
	return event
}
