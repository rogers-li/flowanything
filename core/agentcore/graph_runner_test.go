package agentcore

import (
	"context"
	"strings"
	"testing"
)

func TestGraphRunnerExposesChildAgentsAsCapabilities(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[{"type":"agent","id":"research","task":"research AI news","input":{"user_request":"AI news"},"reason":"needs research"}]}`,
		"AI news summary from child.",
		"Final summary from parent.",
	}}
	graphRunner := NewGraphRunner(model)

	result, err := graphRunner.Run(context.Background(), AgentGraphRunRequest{
		TraceID:     "trace_graph",
		UserMessage: "Summarize AI news",
		Graph: AgentGraphSpec{
			ID:          "flow_news",
			EntryNodeID: "start",
			Nodes: []AgentGraphNode{
				{ID: "start", Type: "start"},
				{
					ID:   "supervisor",
					Type: "supervisor_node",
					Agent: AgentSpec{
						ID:            "agent_supervisor",
						Name:          "Supervisor",
						Prompt:        "Route to the best sub-agent.",
						ReasoningMode: "action-planning",
						Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
					},
				},
				{
					ID:   "research",
					Type: "agent_node",
					Agent: AgentSpec{
						ID:            "agent_research",
						Name:          "Research Agent",
						Prompt:        "Research the requested topic.",
						ReasoningMode: "direct",
						Model:         ModelConfig{Provider: "fake", Model: "fake-model"},
					},
				},
			},
			Edges: []AgentGraphEdge{
				{From: "start", To: "supervisor"},
				{From: "supervisor", To: "research"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Final summary from parent." {
		t.Fatalf("unexpected final text: %q", result.Text)
	}
	if result.Output["return_message"] != "Final summary from parent." {
		t.Fatalf("expected return_message output, got %#v", result.Output)
	}
	if len(model.requests) != 3 {
		t.Fatalf("expected planning, child, and final model calls, got %d", len(model.requests))
	}
	if !strings.Contains(model.requests[0].Messages[0].Content, "Research Agent") {
		t.Fatalf("sub-agent capability was not included in planning prompt: %s", model.requests[0].Messages[0].Content)
	}
}

func TestGraphRunnerDefaultsGraphNodesToReWOO(t *testing.T) {
	model := &fakeModel{responses: []string{
		`{"actions":[],"final_answer_if_no_action":"Handled by default ReWOO."}`,
	}}
	events := &MemoryEventSink{}
	graphRunner := NewGraphRunner(model, WithGraphEventHook(events))

	result, err := graphRunner.Run(context.Background(), AgentGraphRunRequest{
		TraceID:     "trace_graph_rewoo_default",
		UserMessage: "hello",
		Graph: AgentGraphSpec{
			ID:          "flow_default_rewoo",
			EntryNodeID: "start",
			Nodes: []AgentGraphNode{
				{ID: "start", Type: "start"},
				{
					ID:   "agent",
					Type: "agent_node",
					Agent: AgentSpec{
						ID:     "agent_default",
						Name:   "Default Agent",
						Prompt: "Answer directly when no action is needed.",
						Model:  ModelConfig{Provider: "fake", Model: "fake-model"},
					},
				},
			},
			Edges: []AgentGraphEdge{{From: "start", To: "agent"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Handled by default ReWOO." {
		t.Fatalf("unexpected result: %q", result.Text)
	}
	if len(model.requests) != 1 {
		t.Fatalf("expected one ReWOO planning call, got %d", len(model.requests))
	}
	event := findAgentEvent(events.Events, EventAgentStarted)
	if event == nil || event.Strategy != "rewoo" {
		t.Fatalf("expected graph node to default to rewoo strategy, got %#v", event)
	}
}

func TestGraphRunnerRejectsMultipleStartChildren(t *testing.T) {
	graphRunner := NewGraphRunner(&fakeModel{})
	_, err := graphRunner.Run(context.Background(), AgentGraphRunRequest{
		Graph: AgentGraphSpec{
			ID:          "flow_invalid",
			EntryNodeID: "start",
			Nodes: []AgentGraphNode{
				{ID: "start", Type: "start"},
				{ID: "left", Type: "agent_node", Agent: AgentSpec{ID: "left", Prompt: "x", Model: ModelConfig{Provider: "fake", Model: "fake"}}},
				{ID: "right", Type: "agent_node", Agent: AgentSpec{ID: "right", Prompt: "x", Model: ModelConfig{Provider: "fake", Model: "fake"}}},
			},
			Edges: []AgentGraphEdge{
				{From: "start", To: "left"},
				{From: "start", To: "right"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one outgoing agent node") {
		t.Fatalf("expected start uniqueness error, got %v", err)
	}
}

func TestGraphRunnerRejectsCyclesBeforeModelCall(t *testing.T) {
	model := &fakeModel{}
	graphRunner := NewGraphRunner(model)
	_, err := graphRunner.Run(context.Background(), AgentGraphRunRequest{
		Graph: AgentGraphSpec{
			ID:          "flow_cycle",
			EntryNodeID: "start",
			Nodes: []AgentGraphNode{
				{ID: "start", Type: "start"},
				{ID: "supervisor", Type: "agent_node", Agent: AgentSpec{ID: "supervisor", Prompt: "x", Model: ModelConfig{Provider: "fake", Model: "fake"}}},
				{ID: "worker", Type: "agent_node", Agent: AgentSpec{ID: "worker", Prompt: "x", Model: ModelConfig{Provider: "fake", Model: "fake"}}},
			},
			Edges: []AgentGraphEdge{
				{From: "start", To: "supervisor"},
				{From: "supervisor", To: "worker"},
				{From: "worker", To: "supervisor"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cycle detected") {
		t.Fatalf("expected cycle error, got %v", err)
	}
	if len(model.requests) != 0 {
		t.Fatalf("cycle validation should fail before model calls, got %d calls", len(model.requests))
	}
}
