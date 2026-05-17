package flowengine

import "testing"

func TestDataContextSupportsGenericNodeContext(t *testing.T) {
	data := NewDataContext(map[string]any{"query": "ai news"})

	if err := data.Write("$.node_context.connector.responses.tavily_search", map[string]any{
		"status_code": 200,
		"body": map[string]any{
			"answer": "latest news",
		},
	}); err != nil {
		t.Fatal(err)
	}
	got, ok := data.Read("$.node_context.connector.responses.tavily_search.body.answer")
	if !ok || got != "latest news" {
		t.Fatalf("unexpected connector node context value: %#v, ok=%v", got, ok)
	}

	if err := data.WriteNodeContext("agent", "scratch.plan", []string{"search", "summarize"}); err != nil {
		t.Fatal(err)
	}
	plan, ok := data.Read("$.node_context.agent.scratch.plan")
	if !ok {
		t.Fatal("expected agent node context value")
	}
	items := plan.([]string)
	if len(items) != 2 || items[0] != "search" {
		t.Fatalf("unexpected agent node context value: %#v", plan)
	}
}
