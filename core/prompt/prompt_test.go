package prompt

import (
	"strings"
	"testing"

	"flow-anything/core/capability"
	"flow-anything/core/schema"
)

func TestBuildPlanningSystemPromptIncludesCapabilitiesAndSchemas(t *testing.T) {
	caps := FromCapabilityDescriptors([]capability.Descriptor{{
		ID:          "tool_search",
		Kind:        capability.KindTool,
		Name:        "Search",
		Description: "Search the web",
		InputSchema: schema.Schema{{
			Name:        "query",
			Type:        schema.TypeString,
			Required:    true,
			Description: "Search query",
		}},
	}})
	text := BuildPlanningSystemPrompt(PlanningPromptRequest{
		AgentName:        "Research Agent",
		AgentDescription: "Finds and summarizes information",
		Base:             Spec{System: "You are careful."},
		Capabilities:     caps,
		OutputSchema: schema.Schema{{
			Name: "summary",
			Type: schema.TypeString,
		}},
	})
	for _, expected := range []string{
		"You are careful.",
		"Runtime Action Planning Contract",
		"type=tool; id=tool_search",
		"$.query",
		"$.summary",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("planning prompt missing %q:\n%s", expected, text)
		}
	}
}

func TestBuildFinalAnswerSystemPromptRequiresJSONWhenSchemaExists(t *testing.T) {
	text := BuildFinalAnswerSystemPrompt(Spec{System: "You route requests."}, schema.Schema{{
		Name:        "next_node_ids",
		Type:        schema.TypeArray,
		Required:    true,
		Description: "Next nodes.",
	}})

	for _, expected := range []string{
		"Structured output is required.",
		"Return exactly one valid JSON object and nothing else.",
		"Do not use markdown fences",
		"@agent(...)",
		"$.next_node_ids",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("final answer prompt missing %q:\n%s", expected, text)
		}
	}
}

func TestRenderTemplateReplacesVariablesDeterministically(t *testing.T) {
	text := RenderTemplate("Hello {{name}}, run {{task}}.", map[string]any{
		"task": "search",
		"name": "Codex",
	})
	if text != "Hello Codex, run search." {
		t.Fatalf("unexpected rendered template: %s", text)
	}
}
