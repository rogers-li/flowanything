package agentcore

import (
	"context"
	"strings"
	"testing"

	"flow-anything/core/runtimecontext"
)

type fakeModel struct {
	responses     []string
	requests      []ModelRequest
	traceContexts []runtimecontext.TraceContext
}

func (m *fakeModel) Chat(ctx Context, req ModelRequest) (ModelResponse, error) {
	m.requests = append(m.requests, req)
	if traceContext, ok := runtimecontext.TraceContextFrom(ctx); ok {
		m.traceContexts = append(m.traceContexts, traceContext)
	}
	content := ""
	if len(m.responses) > 0 {
		content = m.responses[0]
		m.responses = m.responses[1:]
	}
	return ModelResponse{Message: Message{Role: "assistant", Content: content}}, nil
}

func TestDirectStrategyCallsModelOnce(t *testing.T) {
	model := &fakeModel{responses: []string{"hello"}}
	runner := NewRunner(model)

	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_direct",
		UserMessage: "hi",
		Agent: AgentSpec{
			ID:            "agent_direct",
			Prompt:        "You are concise.",
			ReasoningMode: "direct",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "hello" {
		t.Fatalf("unexpected answer: %q", result.Text)
	}
	if len(model.requests) != 1 {
		t.Fatalf("expected one model call, got %d", len(model.requests))
	}
	if model.requests[0].Messages[0].Content != "You are concise." {
		t.Fatalf("system prompt was not forwarded")
	}
}

func TestActionPlanningStrategyInvokesCapabilityAndFinalizes(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_weather","task":"query weather","input":{"city":"Shanghai"},"reason":"user asked weather"}]}`,
		"Shanghai is sunny.",
	}}
	capabilities := NewMapCapabilityRegistry()
	calls := 0
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_weather",
			Type:        "tool",
			Name:        "Weather Tool",
			Description: "Query weather.",
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			calls++
			if call.Input["city"] != "Shanghai" {
				t.Fatalf("unexpected capability input: %#v", call.Input)
			}
			if call.TraceContext.SpanID != runtimecontext.AgentCapabilitySpanID("trace_plan", "agent_weather", "tool", "tool_weather") {
				t.Fatalf("capability call should carry capability span: %#v", call.TraceContext)
			}
			return CapabilityResult{
				ID:     call.ID,
				Type:   call.Type,
				Text:   "sunny",
				Output: map[string]any{"weather": "sunny"},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	events := &MemoryEventSink{}
	runner := NewRunner(model, WithCapabilities(capabilities), WithEventSink(events))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_plan",
		UserMessage: "How is weather in Shanghai?",
		Agent: AgentSpec{
			ID:            "agent_weather",
			Prompt:        "You help with weather.",
			ReasoningMode: "action-planning",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_weather",
				Type:        "tool",
				Name:        "Weather Tool",
				Description: "Query weather.",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Shanghai is sunny." {
		t.Fatalf("unexpected final answer: %q", result.Text)
	}
	if calls != 1 {
		t.Fatalf("expected one capability call, got %d", calls)
	}
	if len(model.requests) != 2 {
		t.Fatalf("expected planning and final model calls, got %d", len(model.requests))
	}
	if model.requests[0].TraceContext.SpanID != runtimecontext.AgentModelSpanID("trace_plan", "agent_weather", "planning") {
		t.Fatalf("planning request should carry planning span: %#v", model.requests[0].TraceContext)
	}
	if model.requests[0].TraceContext.ParentSpanID != runtimecontext.AgentPlanningSpanID("trace_plan", "agent_weather") {
		t.Fatalf("planning model request should be a child of planning span: %#v", model.requests[0].TraceContext)
	}
	if model.requests[1].TraceContext.SpanID != runtimecontext.AgentModelSpanID("trace_plan", "agent_weather", "final_answer") {
		t.Fatalf("final answer request should carry final-answer span: %#v", model.requests[1].TraceContext)
	}
	if model.requests[1].TraceContext.ParentSpanID != runtimecontext.AgentFinalAnswerSpanID("trace_plan", "agent_weather") {
		t.Fatalf("final model request should be a child of final-answer span: %#v", model.requests[1].TraceContext)
	}
	if len(model.traceContexts) != 2 || model.traceContexts[0].SpanID != runtimecontext.AgentModelSpanID("trace_plan", "agent_weather", "planning") {
		t.Fatalf("model ctx should carry trace context: %#v", model.traceContexts)
	}
	if len(result.Actions) != 1 || result.Actions[0].Action.ID != "tool_weather" {
		t.Fatalf("unexpected action results: %#v", result.Actions)
	}
	if len(events.Events) == 0 {
		t.Fatalf("expected agent events")
	}
	if !hasAgentEvent(events.Events, EventPlanningCompleted) {
		t.Fatalf("expected planning completed event: %#v", events.Events)
	}
	if !hasAgentEvent(events.Events, EventCapabilityCompleted) {
		t.Fatalf("expected capability completed event: %#v", events.Events)
	}
	if !hasAgentEvent(events.Events, EventFinalAnswerCompleted) {
		t.Fatalf("expected final answer completed event: %#v", events.Events)
	}
}

func TestReWOOStrategyPlansExecutesAndSolvesWithoutObservationLoop(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_search","task":"search AI news","input":{"query":"AI news"},"reason":"needs fresh information"}]}`,
		"Final answer from one ReWOO solve.",
	}}
	capabilities := NewMapCapabilityRegistry()
	calls := 0
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_search",
			Type:        "tool",
			Name:        "Search Tool",
			Description: "Search web.",
			InputSchema: []SchemaField{{
				Name:     "query",
				Type:     "string",
				Required: true,
			}},
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			calls++
			return CapabilityResult{
				ID:     call.ID,
				Type:   call.Type,
				Text:   "search observation",
				Output: map[string]any{"summary": "AI news"},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	events := &MemoryEventSink{}
	runner := NewRunner(model, WithCapabilities(capabilities), WithEventSink(events))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_rewoo",
		UserMessage: "Search AI news",
		Agent: AgentSpec{
			ID:            "agent_research",
			Prompt:        "You research topics.",
			ReasoningMode: "rewoo",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_search",
				Type:        "tool",
				Name:        "Search Tool",
				Description: "Search web.",
				InputSchema: []SchemaField{{
					Name:     "query",
					Type:     "string",
					Required: true,
				}},
			}},
			Policy: AgentPolicy{MaxIterations: 8, MaxActions: 8},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Final answer from one ReWOO solve." {
		t.Fatalf("unexpected final answer: %q", result.Text)
	}
	if calls != 1 {
		t.Fatalf("expected one capability call, got %d", calls)
	}
	if len(model.requests) != 2 {
		t.Fatalf("ReWOO should only call planning and final answer models, got %d", len(model.requests))
	}
	for _, eventType := range []AgentEventType{EventPlanningStarted, EventPlanningCompleted, EventFinalAnswerCompleted} {
		event := findAgentEvent(events.Events, eventType)
		if event == nil || event.Strategy != "rewoo" {
			t.Fatalf("expected %s event with rewoo strategy, got %#v", eventType, event)
		}
	}
}

func TestSkillCapabilityRunsSkillWithReWOOAndPrivateTools(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_search","task":"search AI news","input":{"query":"AI news"},"reason":"needs fresh information"}]}`,
		"skill final answer",
	}}
	capabilities := NewMapCapabilityRegistry()
	toolCalls := 0
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_search",
			Type:        "tool",
			Name:        "Search Tool",
			Description: "Search web.",
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			toolCalls++
			if call.Input["query"] != "AI news" {
				t.Fatalf("unexpected tool input: %#v", call.Input)
			}
			return CapabilityResult{
				ID:     call.ID,
				Type:   call.Type,
				Text:   "search observation",
				Output: map[string]any{"summary": "AI news"},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	events := &MemoryEventSink{}
	runner := NewRunner(model, WithCapabilities(capabilities), WithEventSink(events))
	skill := NewSkillCapability(SkillSpec{
		ID:            "skill_web_search",
		Name:          "Web Search",
		Description:   "Search and summarize web information.",
		Prompt:        "Use the search tool and summarize the result.",
		ReasoningMode: ReWOOStrategy{}.Name(),
		Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
		Capabilities: []CapabilityDescriptor{{
			ID:          "tool_search",
			Type:        "tool",
			Name:        "Search Tool",
			Description: "Search web.",
		}},
		Policy: AgentPolicy{MaxActions: 3, MaxIterations: 3},
	}, runner)

	result, err := skill.Invoke(context.Background(), CapabilityCall{
		ID:      "skill_web_search",
		Type:    "skill",
		Task:    "Find AI news",
		Input:   map[string]any{"query": "AI news"},
		TraceID: "trace_skill_rewoo",
		TraceContext: runtimecontext.TraceContext{
			TraceID: "trace_skill_rewoo",
			SpanID:  "parent_capability_span",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "skill final answer" {
		t.Fatalf("unexpected skill result: %q", result.Text)
	}
	if toolCalls != 1 {
		t.Fatalf("expected skill to invoke its tool once, got %d", toolCalls)
	}
	if len(model.requests) != 2 {
		t.Fatalf("skill ReWOO should plan and solve once, got %d model calls", len(model.requests))
	}
	if len(model.requests[0].Tools) != 1 || model.requests[0].Tools[0].ID != "tool_search" {
		t.Fatalf("skill planning should only expose skill tools, got %#v", model.requests[0].Tools)
	}
	if model.requests[0].TraceContext.ParentSpanID == "" {
		t.Fatalf("skill model call should be linked into the parent trace: %#v", model.requests[0].TraceContext)
	}
	if event := findAgentEvent(events.Events, EventPlanningCompleted); event == nil || event.Strategy != "rewoo" {
		t.Fatalf("expected skill planning to use rewoo, got %#v", event)
	}
}

func TestActionPlanningStrategyResolvesCapabilityByName(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"connector","id":"weather_lookup","task":"query weather","input":{"city":"Shanghai"},"reason":"user asked weather"}]}`,
		"Shanghai is sunny.",
	}}
	capabilities := NewMapCapabilityRegistry()
	calls := 0
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_weather",
			Type:        "tool",
			Name:        "weather_lookup",
			Description: "Query weather.",
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			calls++
			if call.ID != "tool_weather" || call.Type != "tool" {
				t.Fatalf("capability call should use normalized descriptor identity, got id=%s type=%s", call.ID, call.Type)
			}
			return CapabilityResult{ID: call.ID, Type: call.Type, Text: "sunny"}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(model, WithCapabilities(capabilities))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_plan_name_alias",
		UserMessage: "How is weather in Shanghai?",
		Agent: AgentSpec{
			ID:            "agent_weather",
			Prompt:        "You help with weather.",
			ReasoningMode: "action-planning",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_weather",
				Type:        "tool",
				Name:        "weather_lookup",
				Description: "Query weather.",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Shanghai is sunny." {
		t.Fatalf("unexpected final answer: %q", result.Text)
	}
	if calls != 1 {
		t.Fatalf("expected one normalized capability call, got %d", calls)
	}
	if len(result.Actions) != 1 || result.Actions[0].Action.ID != "tool_weather" || result.Actions[0].Action.Type != "tool" {
		t.Fatalf("expected normalized action result, got %#v", result.Actions)
	}
}

func TestActionPlanningStrategyDoesNotInvokeUnlistedGlobalCapability(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_weather","task":"query weather","input":{"city":"Shanghai"},"reason":"user asked weather"}]}`,
		"I cannot call a weather tool because none is bound.",
	}}
	capabilities := NewMapCapabilityRegistry()
	calls := 0
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_weather",
			Type:        "tool",
			Name:        "weather_lookup",
			Description: "Query weather.",
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			calls++
			return CapabilityResult{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(model, WithCapabilities(capabilities))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_plan_no_hidden_global",
		UserMessage: "How is weather in Shanghai?",
		Agent: AgentSpec{
			ID:            "agent_weather",
			Prompt:        "You help with weather.",
			ReasoningMode: "action-planning",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("unlisted global capability should not be invoked, got %d calls", calls)
	}
	if len(result.Actions) != 1 || result.Actions[0].Error != "capability not found" {
		t.Fatalf("expected unlisted capability observation, got %#v", result.Actions)
	}
}

func TestActionPlanningStrategyPublishesFailureEvents(t *testing.T) {
	model := &fakeModel{responses: []string{"not json"}}
	events := &MemoryEventSink{}
	runner := NewRunner(model, WithEventSink(events))

	_, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_plan_failed",
		UserMessage: "What should I do?",
		Agent: AgentSpec{
			ID:            "agent_failed",
			Prompt:        "You plan actions.",
			ReasoningMode: "action-planning",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !hasAgentEvent(events.Events, EventPlanningFailed) {
		t.Fatalf("expected planning failed event: %#v", events.Events)
	}
	if !hasAgentEvent(events.Events, EventAgentFailed) {
		t.Fatalf("expected agent failed event: %#v", events.Events)
	}
}

func TestReActStrategyLoopsThroughObservationBeforeFinalAnswer(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_search","task":"search AI news","input":{"query":"AI news"},"reason":"need fresh information"}]}`,
		`{"actions":[],"final_answer_if_no_action":"AI news summary is ready."}`,
	}}
	capabilities := NewMapCapabilityRegistry()
	calls := 0
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_search",
			Type:        "tool",
			Name:        "Search Tool",
			Description: "Search web.",
			InputSchema: []SchemaField{{
				Name:     "query",
				Type:     "string",
				Required: true,
			}},
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			calls++
			if call.TraceContext.SpanID != runtimecontext.AgentCapabilitySpanID("trace_react", "agent_research", "tool", "tool_search@1") {
				t.Fatalf("react capability span should include iteration: %#v", call.TraceContext)
			}
			return CapabilityResult{
				ID:     call.ID,
				Type:   call.Type,
				Text:   "found 3 stories",
				Output: map[string]any{"count": 3},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	events := &MemoryEventSink{}
	runner := NewRunner(model, WithCapabilities(capabilities), WithEventSink(events))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_react",
		UserMessage: "Search AI news",
		Agent: AgentSpec{
			ID:            "agent_research",
			Prompt:        "You research topics.",
			ReasoningMode: "react",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_search",
				Type:        "tool",
				Name:        "Search Tool",
				Description: "Search web.",
				InputSchema: []SchemaField{{
					Name:     "query",
					Type:     "string",
					Required: true,
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "AI news summary is ready." {
		t.Fatalf("unexpected react answer: %q", result.Text)
	}
	if calls != 1 {
		t.Fatalf("expected one capability call, got %d", calls)
	}
	if len(model.requests) != 2 {
		t.Fatalf("expected two planning calls, got %d", len(model.requests))
	}
	if !messagesContain(model.requests[1].Messages, "Observations:") {
		t.Fatalf("second react planning request should include observations: %#v", model.requests[1].Messages)
	}
	if !hasAgentEvent(events.Events, EventCapabilityCompleted) {
		t.Fatalf("expected capability completed event: %#v", events.Events)
	}
	expectedCapabilitySpan := runtimecontext.AgentCapabilitySpanID("trace_react", "agent_research", "tool", "tool_search@1")
	if event := findAgentEvent(events.Events, EventCapabilityStarted); event == nil || event.TraceContext.SpanID != expectedCapabilitySpan {
		t.Fatalf("react capability event should use the same span as tool parent, got %#v", event)
	}
}

func TestReActStrategyUsesDistinctSpansForRepeatedCapabilityInSameIteration(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_search","task":"search Anthropic","input":{"query":"Anthropic"},"reason":"needed"},{"type":"tool","id":"tool_search","task":"search Google I/O","input":{"query":"Google I/O"},"reason":"needed"}]}`,
		`{"actions":[],"final_answer_if_no_action":"Combined summary."}`,
	}}
	capabilities := NewMapCapabilityRegistry()
	var spanIDs []string
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_search",
			Type:        "tool",
			Name:        "Search Tool",
			Description: "Search web.",
			InputSchema: []SchemaField{{
				Name:     "query",
				Type:     "string",
				Required: true,
			}},
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			spanIDs = append(spanIDs, call.TraceContext.SpanID)
			return CapabilityResult{
				ID:     call.ID,
				Type:   call.Type,
				Text:   "ok",
				Output: map[string]any{"query": call.Input["query"]},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(model, WithCapabilities(capabilities))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_react_repeated_capability",
		UserMessage: "Compare Anthropic and Google I/O",
		Agent: AgentSpec{
			ID:            "agent_research",
			Prompt:        "You research topics.",
			ReasoningMode: "react",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_search",
				Type:        "tool",
				Name:        "Search Tool",
				Description: "Search web.",
				InputSchema: []SchemaField{{
					Name:     "query",
					Type:     "string",
					Required: true,
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Combined summary." {
		t.Fatalf("unexpected react answer: %q", result.Text)
	}
	expected := []string{
		runtimecontext.AgentCapabilitySpanID("trace_react_repeated_capability", "agent_research", "tool", "tool_search@1.1"),
		runtimecontext.AgentCapabilitySpanID("trace_react_repeated_capability", "agent_research", "tool", "tool_search@1.2"),
	}
	if strings.Join(spanIDs, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected capability spans:\n%v", spanIDs)
	}
}

func TestReActStrategyStopsRepeatedActionLoop(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_search","task":"search AI news","input":{"query":"AI news"},"reason":"need fresh information"}]}`,
		`{"actions":[{"type":"tool","id":"tool_search","task":"search AI news","input":{"query":"AI news"},"reason":"need fresh information"}]}`,
		"Final summary after one search.",
	}}
	capabilities := NewMapCapabilityRegistry()
	calls := 0
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_search",
			Type:        "tool",
			Name:        "Search Tool",
			Description: "Search web.",
			InputSchema: []SchemaField{{
				Name:     "query",
				Type:     "string",
				Required: true,
			}},
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			calls++
			return CapabilityResult{
				ID:     call.ID,
				Type:   call.Type,
				Text:   "found repeated search result",
				Output: map[string]any{"count": 3},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	events := &MemoryEventSink{}
	runner := NewRunner(model, WithCapabilities(capabilities), WithEventSink(events))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_react_loop_guard",
		UserMessage: "Search AI news",
		Agent: AgentSpec{
			ID:            "agent_research",
			Prompt:        "You research topics.",
			ReasoningMode: "react",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_search",
				Type:        "tool",
				Name:        "Search Tool",
				Description: "Search web.",
				InputSchema: []SchemaField{{
					Name:     "query",
					Type:     "string",
					Required: true,
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Final summary after one search." {
		t.Fatalf("unexpected final answer: %q", result.Text)
	}
	if calls != 1 {
		t.Fatalf("duplicate action should be skipped, got %d calls", calls)
	}
	if len(model.requests) != 3 {
		t.Fatalf("expected first plan, repeated plan, and final answer calls, got %d", len(model.requests))
	}
	if !hasAgentEventWithData(events.Events, EventPlanningCompleted, "loop_breaker", true) {
		t.Fatalf("expected loop breaker planning event: %#v", events.Events)
	}
}

func TestRepeatedAgentActionIgnoresTaskWording(t *testing.T) {
	attempted := map[string]struct{}{
		plannedActionFingerprint(PlannedAction{Type: "agent", ID: "agent_research", Task: "search AI news"}): {},
	}
	next, duplicates := filterRepeatedActions([]PlannedAction{{
		Type: "agent",
		ID:   "agent_research",
		Task: "search the latest AI news again with more details",
		Input: map[string]any{
			"user_request": "AI news",
		},
	}}, attempted)
	if len(next) != 0 || len(duplicates) != 1 {
		t.Fatalf("same agent should be treated as duplicate in one ReAct run, next=%#v duplicates=%#v", next, duplicates)
	}
}

func TestRepeatedHighLevelCapabilityIgnoresTaskWording(t *testing.T) {
	for _, capabilityType := range []string{"skill", "workflow"} {
		attempted := map[string]struct{}{
			plannedActionFingerprint(PlannedAction{Type: capabilityType, ID: "cap_web_search", Task: "search AI news"}): {},
		}
		next, duplicates := filterRepeatedActions([]PlannedAction{{
			Type: capabilityType,
			ID:   "cap_web_search",
			Task: "search the latest AI news again with richer details",
			Input: map[string]any{
				"query": "AI news",
			},
		}}, attempted)
		if len(next) != 0 || len(duplicates) != 1 {
			t.Fatalf("same %s should be treated as duplicate in one ReAct run, next=%#v duplicates=%#v", capabilityType, next, duplicates)
		}
	}
}

func TestReActStrategyAcceptsNaturalFinalAnswerAfterObservation(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_search","task":"search weather","input":{"query":"深圳天气"},"reason":"need current weather"}]}`,
		"深圳今天多云，局地有短时阵雨，出门建议带伞。",
	}}
	capabilities := NewMapCapabilityRegistry()
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_search",
			Type:        "tool",
			Name:        "Search Tool",
			Description: "Search web.",
			InputSchema: []SchemaField{{
				Name:     "query",
				Type:     "string",
				Required: true,
			}},
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			return CapabilityResult{
				ID:     call.ID,
				Type:   call.Type,
				Text:   "深圳天气搜索结果",
				Output: map[string]any{"summary": "多云，有短时阵雨"},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	events := &MemoryEventSink{}
	runner := NewRunner(model, WithCapabilities(capabilities), WithEventSink(events))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_react_natural_final",
		UserMessage: "帮我查一下深圳天气",
		Agent: AgentSpec{
			ID:            "agent_weather",
			Prompt:        "You answer weather questions.",
			ReasoningMode: "react",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_search",
				Type:        "tool",
				Name:        "Search Tool",
				Description: "Search web.",
				InputSchema: []SchemaField{{
					Name:     "query",
					Type:     "string",
					Required: true,
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "深圳今天多云，局地有短时阵雨，出门建议带伞。" {
		t.Fatalf("unexpected react answer: %q", result.Text)
	}
	if len(result.Actions) != 1 || result.Actions[0].Action.ID != "tool_search" {
		t.Fatalf("expected one observed action, got %#v", result.Actions)
	}
	if !hasAgentEvent(events.Events, EventFinalAnswerCompleted) {
		t.Fatalf("expected final answer completed event: %#v", events.Events)
	}
	if hasAgentEvent(events.Events, EventPlanningFailed) {
		t.Fatalf("natural final answer after observation should not fail planning: %#v", events.Events)
	}
}

func TestReActStrategyFinalizesMalformedActionPlanAfterObservation(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_search","task":"search AI news","input":{"query":"AI news"},"reason":"need fresh information"}]}`,
		`我先查询新闻列表。

{"actions":[{"type":"tool","id":"tool_search","task":"search AI news","input":{"query":"AI news"},"reason":"need fresh information"}]}`,
		"这是整理后的 AI 新闻摘要。",
	}}
	capabilities := NewMapCapabilityRegistry()
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{ID: "tool_search", Type: "tool", Name: "Search Tool", Description: "Search web."},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			return CapabilityResult{ID: call.ID, Type: call.Type, Text: "found AI news"}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(model, WithCapabilities(capabilities))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_react_malformed_plan_final",
		UserMessage: "Search AI news",
		Agent: AgentSpec{
			ID:            "agent_research",
			Prompt:        "You research topics.",
			ReasoningMode: "react",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities:  []CapabilityDescriptor{{ID: "tool_search", Type: "tool", Name: "Search Tool", Description: "Search web."}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "这是整理后的 AI 新闻摘要。" {
		t.Fatalf("unexpected final answer: %q", result.Text)
	}
	if len(model.requests) != 3 {
		t.Fatalf("expected first plan, malformed plan, and final answer calls, got %d", len(model.requests))
	}
}

func TestStructuredFinalOutputUsesTextFieldAsResultText(t *testing.T) {
	model := &fakeModel{responses: []string{`{"text":"请继续执行 Web Search Agent。","next_node_ids":["agent_web"]}`}}
	runner := NewRunner(model)

	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_structured_output_text",
		UserMessage: "Search AI news",
		Agent: AgentSpec{
			ID:            "agent_router",
			Prompt:        "Route the request.",
			ReasoningMode: "direct",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			OutputSchema: []SchemaField{
				{Name: "text", Type: "string", Required: true},
				{Name: "next_node_ids", Type: "array", Required: true},
			},
			Policy: AgentPolicy{ValidateFinalOutput: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "请继续执行 Web Search Agent。" {
		t.Fatalf("expected text field to be used as result text, got %q", result.Text)
	}
	if got := result.Output["next_node_ids"]; got == nil {
		t.Fatalf("expected structured next_node_ids in output: %#v", result.Output)
	}
}

func TestReActFinalAnswerPromptRequiresStructuredOutput(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"agent","id":"agent_news","task":"search AI news","input":{},"reason":"route to news agent"}]}`,
		`{"text":"交给 News Agent 继续处理。","next_node_ids":["agent_news"]}`,
	}}
	runner := NewRunner(model)

	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_react_structured_output",
		UserMessage: "搜索今天的 AI 新闻",
		Agent: AgentSpec{
			ID:            "agent_router",
			Prompt:        "Route the request to the next node.",
			ReasoningMode: "react",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			OutputSchema: []SchemaField{
				{Name: "text", Type: "string", Required: true},
				{Name: "next_node_ids", Type: "array", Required: true},
			},
			Policy: AgentPolicy{ValidateFinalOutput: true, MaxIterations: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "交给 News Agent 继续处理。" {
		t.Fatalf("expected structured text result, got %q", result.Text)
	}
	if len(model.requests) != 2 {
		t.Fatalf("expected planning and final answer requests, got %d", len(model.requests))
	}
	finalSystemPrompt := model.requests[1].Messages[0].Content
	for _, expected := range []string{
		"Return exactly one valid JSON object and nothing else.",
		"@agent(...)",
		"$.next_node_ids",
	} {
		if !strings.Contains(finalSystemPrompt, expected) {
			t.Fatalf("final answer prompt missing %q:\n%s", expected, finalSystemPrompt)
		}
	}
}

func TestReActPlanningExtractsJSONPlanFromPrefixedModelText(t *testing.T) {
	model := &fakeModel{responses: []string{
		`我会先调用搜索工具。{"actions":[{"type":"tool","id":"tool_search","task":"search fresh AI news","input":{"query":"2026 AI news"},"reason":"need current information"}]}`,
		`{"text":"搜索完成。"}`,
	}}
	calls := 0
	capabilities := NewMapCapabilityRegistry()
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_search",
			Type:        "tool",
			Name:        "Search",
			Description: "Search web.",
			InputSchema: []SchemaField{{
				Name:     "query",
				Type:     "string",
				Required: true,
			}},
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			calls++
			if call.Input["query"] != "2026 AI news" {
				t.Fatalf("unexpected query input: %#v", call.Input)
			}
			return CapabilityResult{ID: call.ID, Type: call.Type, Text: "ok"}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(model, WithCapabilities(capabilities))

	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_prefixed_plan",
		UserMessage: "搜索 2026 AI 新闻",
		Agent: AgentSpec{
			ID:            "agent_search",
			Prompt:        "Use search when fresh information is needed.",
			ReasoningMode: "react",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_search",
				Type:        "tool",
				Name:        "Search",
				Description: "Search web.",
				InputSchema: []SchemaField{{
					Name:     "query",
					Type:     "string",
					Required: true,
				}},
			}},
			OutputSchema: []SchemaField{{
				Name:     "text",
				Type:     "string",
				Required: true,
			}},
			Policy: AgentPolicy{ValidateFinalOutput: true, MaxIterations: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected one capability call, got %d", calls)
	}
	if result.Text != "搜索完成。" {
		t.Fatalf("unexpected result text: %q", result.Text)
	}
}

func TestActionPlanningValidatesCapabilityInputSchema(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"tool","id":"tool_weather","task":"query weather","input":{},"reason":"user asked weather"}]}`,
		"Please provide a city.",
	}}
	capabilities := NewMapCapabilityRegistry()
	calls := 0
	if err := capabilities.Register(CapabilityFunc{
		Desc: CapabilityDescriptor{
			ID:          "tool_weather",
			Type:        "tool",
			Name:        "Weather Tool",
			Description: "Query weather.",
			InputSchema: []SchemaField{{
				Name:     "city",
				Type:     "string",
				Required: true,
			}},
		},
		Fn: func(_ Context, call CapabilityCall) (CapabilityResult, error) {
			calls++
			return CapabilityResult{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	events := &MemoryEventSink{}
	runner := NewRunner(model, WithCapabilities(capabilities), WithEventSink(events))
	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_validation",
		UserMessage: "How is the weather?",
		Agent: AgentSpec{
			ID:            "agent_weather",
			Prompt:        "You help with weather.",
			ReasoningMode: "action-planning",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			Capabilities: []CapabilityDescriptor{{
				ID:          "tool_weather",
				Type:        "tool",
				Name:        "Weather Tool",
				Description: "Query weather.",
				InputSchema: []SchemaField{{
					Name:     "city",
					Type:     "string",
					Required: true,
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("invalid action input should not invoke capability, got %d calls", calls)
	}
	if len(result.Actions) != 1 || !strings.Contains(result.Actions[0].Error, "required field is missing") {
		t.Fatalf("expected validation error observation, got %#v", result.Actions)
	}
	if !hasAgentEvent(events.Events, EventCapabilityFailed) {
		t.Fatalf("expected capability failed event: %#v", events.Events)
	}
}

func TestDirectStrategyValidatesStructuredFinalOutputWhenEnabled(t *testing.T) {
	model := &fakeModel{responses: []string{`{"text":"hello"}`}}
	runner := NewRunner(model)

	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_output",
		UserMessage: "hi",
		Agent: AgentSpec{
			ID:            "agent_structured",
			Prompt:        "Return JSON.",
			ReasoningMode: "direct",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			OutputSchema: []SchemaField{{
				Name:     "text",
				Type:     "string",
				Required: true,
			}},
			Policy: AgentPolicy{ValidateFinalOutput: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["text"] != "hello" {
		t.Fatalf("expected parsed structured output, got %#v", result.Output)
	}
	if len(model.requests) != 1 || len(model.requests[0].Messages) == 0 {
		t.Fatalf("expected direct model request to be captured")
	}
	systemPrompt := model.requests[0].Messages[0].Content
	if !strings.Contains(systemPrompt, "Return exactly one valid JSON object and nothing else.") {
		t.Fatalf("direct strategy should instruct structured final output, got:\n%s", systemPrompt)
	}
}

func TestDirectStrategyExtractsStructuredFinalOutputFromPrefixedText(t *testing.T) {
	model := &fakeModel{responses: []string{`好的，结果如下：{"text":"hello"}`}}
	runner := NewRunner(model)

	result, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_output_prefix",
		UserMessage: "hi",
		Agent: AgentSpec{
			ID:            "agent_structured",
			Prompt:        "Return JSON.",
			ReasoningMode: "direct",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
			OutputSchema: []SchemaField{{
				Name:     "text",
				Type:     "string",
				Required: true,
			}},
			Policy: AgentPolicy{ValidateFinalOutput: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "hello" {
		t.Fatalf("expected extracted structured output text, got %q", result.Text)
	}
}

func hasAgentEvent(events []AgentEvent, eventType AgentEventType) bool {
	return findAgentEvent(events, eventType) != nil
}

func findAgentEvent(events []AgentEvent, eventType AgentEventType) *AgentEvent {
	for _, event := range events {
		if event.Type == eventType {
			return &event
		}
	}
	return nil
}

func hasAgentEventWithData(events []AgentEvent, eventType AgentEventType, key string, expected any) bool {
	for _, event := range events {
		if event.Type != eventType || event.Data == nil {
			continue
		}
		if event.Data[key] == expected {
			return true
		}
	}
	return false
}

func messagesContain(messages []Message, text string) bool {
	for _, message := range messages {
		if strings.Contains(message.Content, text) {
			return true
		}
	}
	return false
}
