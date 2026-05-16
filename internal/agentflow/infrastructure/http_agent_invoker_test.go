package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/kernel/id"
)

func TestHTTPAgentInvokerPostsEventToOrchestrator(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/events" {
			t.Fatalf("path = %s, want /v1/events", r.URL.Path)
		}

		var evt event.Event
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		if evt.AgentID != "agent_weather" {
			t.Fatalf("AgentID = %s, want agent_weather", evt.AgentID)
		}
		if evt.Payload["text"] != "北京天气怎么样" {
			t.Fatalf("payload text = %v, want 北京天气怎么样", evt.Payload["text"])
		}
		if evt.Payload[runtimeSystemPromptPayloadKey] != "runtime rules" {
			t.Fatalf("runtime system prompt = %v, want runtime rules", evt.Payload[runtimeSystemPromptPayloadKey])
		}

		body, err := json.Marshal(event.Response{
			EventID: evt.ID,
			TraceID: evt.TraceID,
			Actions: []event.Action{
				{Type: event.ActionSpeak, Text: "北京晴"},
				{Type: event.ActionEndTurn},
			},
		})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}

	invoker := NewHTTPAgentInvokerWithClient("http://orchestrator.test", client)
	result, err := invoker.InvokeAgent(context.Background(), ports.AgentInvocationRequest{
		Run: domain.FlowRun{
			ID:       "run_test",
			TenantID: "tenant_1",
		},
		AgentID: id.ID("agent_weather"),
		Payload: map[string]any{
			runtimeSystemPromptPayloadKey: "untrusted payload value",
		},
		RuntimeSystemPrompt: "runtime rules",
		Task:                "北京天气怎么样",
	})
	if err != nil {
		t.Fatalf("InvokeAgent() error = %v", err)
	}
	if result.Text != "北京晴" {
		t.Fatalf("Text = %q, want 北京晴", result.Text)
	}
	if result.TraceID == "" {
		t.Fatal("TraceID is empty")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
