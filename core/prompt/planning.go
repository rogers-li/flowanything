package prompt

import (
	"strings"

	"flow-anything/core/schema"
)

func BuildPlanningSystemPrompt(req PlanningPromptRequest) string {
	var builder strings.Builder
	base := BuildText(req.Base)
	if base != "" {
		builder.WriteString(base)
		builder.WriteString("\n\n")
	}
	if req.AgentName != "" || req.AgentDescription != "" {
		builder.WriteString("Current Agent:\n")
		if req.AgentName != "" {
			builder.WriteString("- name: ")
			builder.WriteString(req.AgentName)
			builder.WriteString("\n")
		}
		if req.AgentDescription != "" {
			builder.WriteString("- description: ")
			builder.WriteString(req.AgentDescription)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("Runtime Action Planning Contract:\n")
	builder.WriteString("基于当前 Agent 的提示词、用户请求和可用能力，规划下一步要执行的 action list。Tools、Skills、Agents、Workflows 都是同一类可调用 action；只有当需要它们时才放入 actions。\n")
	builder.WriteString("Return JSON only, with no markdown.\n")
	builder.WriteString(`Schema: {"actions":[{"type":"tool|skill|agent|workflow|knowledge","id":"...","task":"specific task","input":{},"reason":"why this action is needed"}],"final_answer_if_no_action":"optional final answer"}`)
	builder.WriteString("\nConstraints:\n")
	builder.WriteString("- Select only from the available capabilities below.\n")
	builder.WriteString("- Keep each action task self-contained and concise.\n")
	builder.WriteString("- For tool or workflow actions, put arguments in input.\n")
	builder.WriteString("- If no capability is needed, return an empty actions array and final_answer_if_no_action.\n")
	builder.WriteString("- Do not call capabilities directly in the planning response; only return the JSON plan.\n")
	builder.WriteString("\nAvailable capabilities:\n")
	builder.WriteString(DescribeCapabilities(req.Capabilities))
	if len(req.OutputSchema) > 0 {
		builder.WriteString("\n\nExpected final output schema:\n")
		builder.WriteString(schema.Describe(req.OutputSchema))
	}
	return strings.TrimSpace(builder.String())
}

func BuildFinalAnswerSystemPrompt(base Spec, outputSchema schema.Schema) string {
	var builder strings.Builder
	builder.WriteString(BuildText(base))
	if builder.Len() > 0 {
		builder.WriteString("\n\n")
	}
	builder.WriteString("Use the observations to answer the user naturally and accurately.")
	if len(outputSchema) > 0 {
		builder.WriteString("\nIf structured output is required, follow this schema:\n")
		builder.WriteString(schema.Describe(outputSchema))
	}
	return strings.TrimSpace(builder.String())
}
