package capability

import (
	"context"
	"strings"
	"testing"
)

func TestRegistryFiltersAndInvokesCapabilities(t *testing.T) {
	registry, err := NewMapRegistry(CapabilityFunc{
		Desc: Descriptor{ID: "tool_search", Kind: KindTool, Name: "Search"},
		Fn: func(ctx context.Context, call Call) (Result, error) {
			return Result{ID: call.ID, Kind: call.Kind, Text: "ok"}, nil
		},
	}, CapabilityFunc{
		Desc: Descriptor{ID: "agent_weather", Kind: KindAgent, Name: "Weather", Disabled: true},
		Fn: func(ctx context.Context, call Call) (Result, error) {
			return Result{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	listed := registry.List(Filter{})
	if len(listed) != 1 || listed[0].ID != "tool_search" {
		t.Fatalf("unexpected listed capabilities: %#v", listed)
	}
	cap, ok := registry.Get("tool_search")
	if !ok {
		t.Fatal("expected capability")
	}
	result, err := cap.Invoke(context.Background(), Call{ID: "tool_search", Kind: KindTool})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "ok" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseActionPlanStripsMarkdownFence(t *testing.T) {
	plan, err := ParseActionPlan("```json\n{\"actions\":[{\"type\":\"tool\",\"id\":\"tool_search\",\"task\":\"search\",\"reason\":\"needed\"}]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != KindTool {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if strings.TrimSpace(plan.Actions[0].Reason) == "" {
		t.Fatal("expected reason")
	}
}

func TestParseActionPlanExtractsPrefixedJSON(t *testing.T) {
	plan, err := ParseActionPlan(`明白了，我会调用搜索工具。{"actions":[{"type":"tool","id":"tool_search","task":"search 2026 AI news","input":{"query":"2026 AI news"},"reason":"needs fresh info"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].ID != "tool_search" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if plan.Actions[0].Input["query"] != "2026 AI news" {
		t.Fatalf("unexpected action input: %#v", plan.Actions[0].Input)
	}
}

func TestParseActionPlanAcceptsStructuredFinalAnswerObject(t *testing.T) {
	plan, err := ParseActionPlan(`{"actions":[],"final_answer_if_no_action":{"text":"done"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if plan.FinalAnswerIfNoAction != `{"text":"done"}` {
		t.Fatalf("unexpected final answer: %q", plan.FinalAnswerIfNoAction)
	}
}
