package agentcore

import (
	"context"
	"strings"
	"testing"
)

type fakeMemoryProvider struct {
	items []MemoryItem
	calls int
}

func (p *fakeMemoryProvider) Recall(ctx Context, req MemoryRecallRequest) ([]MemoryItem, error) {
	p.calls++
	if req.Limit <= 0 || len(p.items) <= req.Limit {
		return p.items, nil
	}
	return p.items[:req.Limit], nil
}

func TestDefaultContextAssemblerIncludesMemoryContextAndBoundedHistory(t *testing.T) {
	assembler := NewDefaultContextAssembler()
	assembly, err := assembler.Assemble(context.Background(), ContextAssemblyRequest{
		Agent: AgentSpec{
			ID: "agent_context",
			Policy: AgentPolicy{
				MaxHistoryMessages: 1,
				MaxContextTokens:   2000,
				MaxMemoryItems:     1,
				MaxMessageChars:    16,
			},
		},
		Phase:        ContextPhasePlanning,
		SystemPrompt: "system",
		UserMessage:  "current",
		Conversation: []Message{
			{Role: "user", Content: "old"},
			{Role: "assistant", Content: "recent"},
		},
		Context: map[string]any{"locale": "zh-CN"},
		Memories: []MemoryItem{
			{ID: "m1", Type: "profile", Content: "likes concise answers"},
			{ID: "m2", Type: "task", Content: "old task"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if assembly.Report.IncludedHistoryCount != 1 || assembly.Report.DroppedHistoryCount != 0 {
		t.Fatalf("expected one bounded history message, got %#v", assembly.Report)
	}
	if assembly.Report.IncludedMemoryCount != 1 || assembly.Report.DroppedMemoryCount != 1 {
		t.Fatalf("expected bounded memory report, got %#v", assembly.Report)
	}
	if !messagesContain(assembly.Messages, "Relevant memory") || !messagesContain(assembly.Messages, "Runtime context") {
		t.Fatalf("expected memory and runtime context messages: %#v", assembly.Messages)
	}
	if !messagesContain(assembly.Messages, "[Context truncated:") {
		t.Fatalf("expected memory content to be truncated by max message chars: %#v", assembly.Messages)
	}
}

func TestRunnerUsesMemoryProviderDuringDirectStrategy(t *testing.T) {
	model := &fakeModel{responses: []string{"hello"}}
	memories := &fakeMemoryProvider{items: []MemoryItem{{ID: "m1", Type: "profile", Content: "user prefers Chinese"}}}
	events := &MemoryEventSink{}
	runner := NewRunner(model, WithMemoryProvider(memories), WithEventSink(events))

	_, err := runner.Run(context.Background(), AgentRunRequest{
		TraceID:     "trace_memory",
		UserMessage: "hi",
		Context:     map[string]any{"channel": "debug"},
		Agent: AgentSpec{
			ID:            "agent_memory",
			Prompt:        "You are helpful.",
			ReasoningMode: "direct",
			Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if memories.calls != 1 {
		t.Fatalf("expected memory recall once, got %d", memories.calls)
	}
	if len(model.requests) != 1 || !messagesContain(model.requests[0].Messages, "user prefers Chinese") {
		t.Fatalf("expected model request to include recalled memory: %#v", model.requests)
	}
	if !hasAgentEvent(events.Events, EventContextAssembled) {
		t.Fatalf("expected context assembled event: %#v", events.Events)
	}
}

func TestDefaultContextAssemblerDropsHistoryWhenBudgetIsSmall(t *testing.T) {
	assembler := NewDefaultContextAssembler()
	assembly, err := assembler.Assemble(context.Background(), ContextAssemblyRequest{
		Agent: AgentSpec{
			ID: "agent_budget",
			Policy: AgentPolicy{
				MaxHistoryMessages: 10,
				MaxContextTokens:   2,
				MaxMessageChars:    1000,
			},
		},
		SystemPrompt: "system prompt consumes budget",
		UserMessage:  "current request",
		Conversation: []Message{{Role: "user", Content: strings.Repeat("history ", 20)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if assembly.Report.DroppedHistoryCount != 1 {
		t.Fatalf("expected history to be dropped under small budget: %#v", assembly.Report)
	}
}
