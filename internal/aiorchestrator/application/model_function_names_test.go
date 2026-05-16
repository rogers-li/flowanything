package application

import (
	"regexp"
	"testing"

	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
)

var modelFunctionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func TestToModelToolsSanitizesFunctionNames(t *testing.T) {
	specs := []tool.Spec{
		{ID: id.ID("tool_weather"), Name: "query_weather"},
		{ID: id.ID("tool_flow_upload"), Name: "上传 飞书 文档"},
		{ID: id.ID("tool_workflow"), Name: "Workflow Tool: Feishu Upload"},
		{ID: id.ID("tool_duplicate_a"), Name: "foo bar"},
		{ID: id.ID("tool_duplicate_b"), Name: "foo@bar"},
	}

	defs := toModelTools(specs)
	byName := mapToolsByName(specs)
	seen := map[string]bool{}
	for _, def := range defs {
		name := def.Function.Name
		if !modelFunctionNamePattern.MatchString(name) {
			t.Fatalf("function name %q is not model-safe", name)
		}
		if seen[name] {
			t.Fatalf("function name %q is not unique", name)
		}
		seen[name] = true
		if _, ok := byName[name]; !ok {
			t.Fatalf("sanitized function name %q cannot be resolved back to a tool", name)
		}
	}

	if defs[0].Function.Name != "query_weather" {
		t.Fatalf("expected already-safe tool name to be preserved, got %q", defs[0].Function.Name)
	}
}

func TestToModelSkillToolsSanitizesFunctionNames(t *testing.T) {
	specs := []skill.Spec{
		{ID: id.ID("skill_web_search"), Name: "web search"},
		{ID: id.ID("skill_cn"), Name: "知识 搜索"},
	}

	defs := toModelSkillTools(specs)
	byName := mapSkillExecutionBindings(specs, nil)
	for _, def := range defs {
		name := def.Function.Name
		if !modelFunctionNamePattern.MatchString(name) {
			t.Fatalf("skill function name %q is not model-safe", name)
		}
		if _, ok := byName[name]; !ok {
			t.Fatalf("sanitized skill function name %q cannot be resolved back to a skill", name)
		}
	}
}
