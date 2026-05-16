package agentorchestration

import (
	"strings"
	"testing"

	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestParseAndFilterActionPlanNormalizesSupportedActions(t *testing.T) {
	plan, err := ParseActionPlan(`{
		"actions": [
			{"type":"agent","node_id":"search_node","task":"search AI news","reason":"need search"},
			{"type":"tool","tool_id":"tool_search","input":{"query":"AI news"}},
			{"type":"skill","skill_id":"skill_writer","task":"write summary"},
			{"type":"tool","tool_id":"unknown_tool"}
		]
	}`)
	if err != nil {
		t.Fatalf("ParseActionPlan() error = %v", err)
	}

	filtered := FilterActionPlan(plan, ActionRegistry{
		Agents: []AgentActionSpec{
			{NodeID: id.ID("search_node"), AgentID: id.ID("agent_search"), Name: "Search Agent"},
		},
		Tools: []tool.Spec{
			{ID: id.ID("tool_search"), TenantID: tenant.ID("tenant_1"), Name: "search"},
		},
		Skills: []skill.Spec{
			{ID: id.ID("skill_writer"), TenantID: tenant.ID("tenant_1"), Name: "writer"},
		},
	}, 10)

	if len(filtered.Actions) != 3 {
		t.Fatalf("filtered actions = %d, want 3: %#v", len(filtered.Actions), filtered.Actions)
	}
	if filtered.Actions[0].AgentID != "agent_search" {
		t.Fatalf("agent action agent_id = %s, want agent_search", filtered.Actions[0].AgentID)
	}
	if filtered.Actions[1].Name != "search" {
		t.Fatalf("tool action name = %s, want search", filtered.Actions[1].Name)
	}
	if filtered.Actions[2].Name != "writer" {
		t.Fatalf("skill action name = %s, want writer", filtered.Actions[2].Name)
	}
}

func TestBuildActionPlanningSystemPromptListsAllActionTypes(t *testing.T) {
	prompt := BuildActionPlanningSystemPrompt(ActionRegistry{
		Agents: []AgentActionSpec{
			{NodeID: id.ID("agent_node"), AgentID: id.ID("agent_weather"), Name: "Weather Agent", Description: "weather"},
		},
		Tools: []tool.Spec{
			{ID: id.ID("tool_weather"), Name: "query_weather", LLMDescription: "query weather"},
		},
		Skills: []skill.Spec{
			{ID: id.ID("skill_weather"), Name: "weather_service", Description: "weather skill"},
		},
		AuthoredPrompt: "天气问题请交给 @agent(Weather Agent) 处理。",
	}, "")

	for _, expected := range []string{
		"authoritative runtime capability registry",
		"Never claim that a listed action is unavailable",
		"Available Tool Actions",
		"tool_weather",
		"Available Skill Actions",
		"skill_weather",
		"Available Agent Actions",
		"agent_weather",
		"Resolved Prompt Agent Mentions",
		"@agent(Weather Agent) => type=agent; node_id=agent_node; agent_id=agent_weather",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q:\n%s", expected, prompt)
		}
	}
}

func TestResolvedPromptAgentMentionsIgnoresUnknownAgents(t *testing.T) {
	prompt := BuildActionPlanningSystemPrompt(ActionRegistry{
		Agents: []AgentActionSpec{
			{NodeID: id.ID("known_node"), AgentID: id.ID("agent_known"), Name: "Known Agent"},
		},
		AuthoredPrompt: "Use @agent(Known Agent), skip @agent(Missing Agent).",
	}, "")

	if !strings.Contains(prompt, "@agent(Known Agent) => type=agent; node_id=known_node; agent_id=agent_known") {
		t.Fatalf("prompt missing known resolved mention:\n%s", prompt)
	}
	if strings.Contains(prompt, "@agent(Missing Agent) =>") {
		t.Fatalf("prompt should not resolve unknown mention:\n%s", prompt)
	}
}

func TestFilterActionPlanHonorsMaxActions(t *testing.T) {
	plan := ActionPlan{
		Actions: []Action{
			{Type: ActionKindTool, ToolID: id.ID("tool_a")},
			{Type: ActionKindTool, ToolID: id.ID("tool_b")},
		},
	}
	filtered := FilterActionPlan(plan, ActionRegistry{
		Tools: []tool.Spec{
			{ID: id.ID("tool_a"), Name: "a"},
			{ID: id.ID("tool_b"), Name: "b"},
		},
	}, 1)
	if len(filtered.Actions) != 1 || filtered.Actions[0].ToolID != "tool_a" {
		t.Fatalf("filtered actions = %#v, want only tool_a", filtered.Actions)
	}
}
