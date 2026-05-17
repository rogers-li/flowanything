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
	if model.requests[0].TraceContext.SpanID != runtimecontext.AgentPlanningSpanID("trace_plan", "agent_weather") {
		t.Fatalf("planning request should carry planning span: %#v", model.requests[0].TraceContext)
	}
	if model.requests[1].TraceContext.SpanID != runtimecontext.AgentFinalAnswerSpanID("trace_plan", "agent_weather") {
		t.Fatalf("final answer request should carry final-answer span: %#v", model.requests[1].TraceContext)
	}
	if len(model.traceContexts) != 2 || model.traceContexts[0].SpanID != runtimecontext.AgentPlanningSpanID("trace_plan", "agent_weather") {
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
}

func hasAgentEvent(events []AgentEvent, eventType AgentEventType) bool {
	for _, event := range events {
		if event.Type == eventType {
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
