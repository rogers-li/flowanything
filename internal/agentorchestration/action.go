package agentorchestration

import (
	"encoding/json"
	"fmt"
	"strings"

	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

const RuntimeDisableToolsPayloadKey = "runtime_disable_tools"

type ActionKind string

const (
	ActionKindTool  ActionKind = "tool"
	ActionKindSkill ActionKind = "skill"
	ActionKindAgent ActionKind = "agent"
)

type ActionPlan struct {
	Actions               []Action `json:"actions"`
	FinalAnswerIfNoAction string   `json:"final_answer_if_no_action,omitempty"`
}

type Action struct {
	Type    ActionKind     `json:"type"`
	ID      id.ID          `json:"id,omitempty"`
	NodeID  id.ID          `json:"node_id,omitempty"`
	AgentID id.ID          `json:"agent_id,omitempty"`
	ToolID  id.ID          `json:"tool_id,omitempty"`
	SkillID id.ID          `json:"skill_id,omitempty"`
	Name    string         `json:"name,omitempty"`
	Task    string         `json:"task,omitempty"`
	Input   map[string]any `json:"input,omitempty"`
	Reason  string         `json:"reason,omitempty"`
}

type ActionObservation struct {
	Type        ActionKind     `json:"type"`
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Task        string         `json:"task,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Text        string         `json:"text,omitempty"`
	TraceID     string         `json:"trace_id,omitempty"`
	Success     bool           `json:"success"`
	Error       string         `json:"error,omitempty"`
	ToolResult  *tool.Result   `json:"tool_result,omitempty"`
	AgentResult any            `json:"agent_result,omitempty"`
}

type AgentActionSpec struct {
	NodeID      id.ID
	AgentID     id.ID
	Name        string
	Description string
}

type ActionRegistry struct {
	Agents         []AgentActionSpec
	Skills         []skill.Spec
	Tools          []tool.Spec
	AuthoredPrompt string
}

type resolvedAgentMention struct {
	MentionName string
	Agent       AgentActionSpec
}

func BuildActionPlanningSystemPrompt(registry ActionRegistry, extraPrompt string) string {
	var builder strings.Builder
	if strings.TrimSpace(extraPrompt) != "" {
		builder.WriteString("Additional Planning Instruction:\n")
		builder.WriteString(strings.TrimSpace(extraPrompt))
		builder.WriteString("\n\n")
	}
	builder.WriteString("Runtime Action Planning Contract:\n")
	builder.WriteString("基于当前 Agent 的系统提示词、用户请求和可用能力，规划下一步要执行的 action list。")
	builder.WriteString("Tools、Skills、Sub-Agents 都是同一类可调用 action；只有当需要它们时才放入 actions。\n")
	builder.WriteString("Important: The Available Actions sections below are the authoritative runtime capability registry. If an action appears there, it is configured and available. Never claim that a listed action is unavailable.\n")
	builder.WriteString("Return JSON only, with no markdown.\n")
	builder.WriteString("Schema: {\"actions\":[{\"type\":\"tool|skill|agent\",\"id\":\"...\",\"name\":\"human readable action name\",\"node_id\":\"...\",\"agent_id\":\"...\",\"tool_id\":\"...\",\"skill_id\":\"...\",\"task\":\"specific task\",\"input\":{},\"reason\":\"why this action is needed\"}],\"final_answer_if_no_action\":\"optional final answer\"}\n")
	builder.WriteString("Agent action example: {\"type\":\"agent\",\"name\":\"copy exact agent name\",\"node_id\":\"copy exact node_id\",\"agent_id\":\"copy exact agent_id\",\"task\":\"self-contained task for that agent\",\"reason\":\"why this agent is needed\"}\n")
	builder.WriteString("Constraints:\n")
	builder.WriteString("- Select only from the available actions below.\n")
	builder.WriteString("- Always include the human readable action name from Available Actions.\n")
	builder.WriteString("- Only say an action is not configured when no matching action appears in Available Actions.\n")
	builder.WriteString("- For agent actions, prefer returning node_id and keep task self-contained.\n")
	builder.WriteString("- For tool actions, put arguments in input and prefer returning tool_id.\n")
	builder.WriteString("- For skill actions, put the skill task in task and prefer returning skill_id.\n")
	builder.WriteString("- If no action is needed, return an empty actions array and final_answer_if_no_action.\n")
	builder.WriteString("- Do not call tools directly in this planning response; only return the JSON plan.\n")
	builder.WriteString("- If the authored Agent prompt routes a request to @agent(Name), and that Name appears in Resolved Prompt Agent Mentions or Available Agent Actions, return an agent action using the exact node_id and agent_id.\n")
	builder.WriteString("\nAvailable Tool Actions:\n")
	for _, spec := range registry.Tools {
		builder.WriteString(fmt.Sprintf(
			"- type=tool; tool_id=%s; id=%s; name=%s; description=%s\n",
			spec.ID,
			spec.ID,
			spec.Name,
			firstNonEmpty(spec.LLMDescription, spec.Description),
		))
	}
	builder.WriteString("\nAvailable Skill Actions:\n")
	for _, spec := range registry.Skills {
		builder.WriteString(fmt.Sprintf(
			"- type=skill; skill_id=%s; id=%s; name=%s; description=%s\n",
			spec.ID,
			spec.ID,
			spec.Name,
			spec.Description,
		))
	}
	builder.WriteString("\nAvailable Agent Actions:\n")
	for _, agent := range registry.Agents {
		builder.WriteString(fmt.Sprintf(
			"- type=agent; node_id=%s; agent_id=%s; id=%s; name=%s; description=%s\n",
			agent.NodeID,
			agent.AgentID,
			agent.AgentID,
			agent.Name,
			agent.Description,
		))
	}
	mentions := resolvedPromptAgentMentions(registry.AuthoredPrompt, registry.Agents)
	if len(mentions) > 0 {
		builder.WriteString("\nResolved Prompt Agent Mentions:\n")
		for _, mention := range mentions {
			builder.WriteString(fmt.Sprintf(
				"- @agent(%s) => type=agent; node_id=%s; agent_id=%s; name=%s\n",
				mention.MentionName,
				mention.Agent.NodeID,
				mention.Agent.AgentID,
				mention.Agent.Name,
			))
		}
	}
	return builder.String()
}

func BuildActionFinalSystemPrompt(extraPrompt string) string {
	var builder strings.Builder
	if strings.TrimSpace(extraPrompt) != "" {
		builder.WriteString("Additional Final Answer Instruction:\n")
		builder.WriteString(strings.TrimSpace(extraPrompt))
		builder.WriteString("\n\n")
	}
	builder.WriteString("Runtime Final Answer Contract:\n")
	builder.WriteString("根据当前 Agent 的 action observations 进行复核、加工和汇总，生成最终用户回复。\n")
	builder.WriteString("Use only the information in the provided action observations. If information is missing, say what is missing.\n")
	return builder.String()
}

func BuildActionPlanningTask(userTask string) string {
	payload, _ := json.MarshalIndent(map[string]any{
		"user_request": userTask,
	}, "", "  ")

	return "Planning data:\n" + string(payload)
}

func BuildActionFinalTask(userTask string, plan ActionPlan, observations any) string {
	payload, _ := json.MarshalIndent(map[string]any{
		"user_request":        userTask,
		"plan":                plan,
		"action_observations": observations,
	}, "", "  ")

	return "Execution data:\n" + string(payload)
}

func BuildSkillActionSystemPrompt(spec skill.Spec, action Action) string {
	var builder strings.Builder
	builder.WriteString("Runtime Skill Action Contract:\n")
	builder.WriteString("Execute the selected Skill as the focused capability for this action.\n")
	builder.WriteString(fmt.Sprintf("Skill ID: %s\n", spec.ID))
	builder.WriteString(fmt.Sprintf("Skill Name: %s\n", spec.Name))
	if strings.TrimSpace(spec.SystemPrompt) != "" {
		builder.WriteString("Skill Prompt:\n")
		builder.WriteString(strings.TrimSpace(spec.SystemPrompt))
		builder.WriteString("\n")
	}
	if len(action.Input) > 0 {
		payload, _ := json.Marshal(action.Input)
		builder.WriteString("Skill Input:\n")
		builder.WriteString(string(payload))
		builder.WriteString("\n")
	}
	builder.WriteString("Execution Rules:\n")
	builder.WriteString("- Use only the selected Skill's available tools.\n")
	builder.WriteString("- If the Skill Prompt describes a sequence, execute the sequence step by step; after each tool result, decide the next required tool call.\n")
	builder.WriteString("- Do not stop after the first tool call unless the skill task is already complete.\n")
	builder.WriteString("- Return the final skill result only after all required tool calls are complete.")
	return builder.String()
}

func ParseActionPlan(text string) (ActionPlan, error) {
	candidate := extractJSONObject(text)
	if candidate == "" {
		return ActionPlan{}, apperrors.New(apperrors.CodeInvalidArgument, "agent action planning response did not contain a json object")
	}

	var plan ActionPlan
	if err := json.Unmarshal([]byte(candidate), &plan); err != nil {
		return ActionPlan{}, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to parse agent action planning json", err)
	}
	for index := range plan.Actions {
		plan.Actions[index] = NormalizeAction(plan.Actions[index])
	}
	plan.FinalAnswerIfNoAction = strings.TrimSpace(plan.FinalAnswerIfNoAction)
	return plan, nil
}

func FilterActionPlan(plan ActionPlan, registry ActionRegistry, maxActions int) ActionPlan {
	result := ActionPlan{FinalAnswerIfNoAction: plan.FinalAnswerIfNoAction}
	for _, action := range plan.Actions {
		action = NormalizeAction(action)
		switch action.Type {
		case ActionKindAgent:
			spec, ok := FindAgentAction(registry.Agents, action)
			if !ok || strings.TrimSpace(action.Task) == "" {
				continue
			}
			action.NodeID = spec.NodeID
			action.AgentID = spec.AgentID
			if action.ID.Empty() {
				action.ID = spec.AgentID
			}
			if action.Name == "" {
				action.Name = spec.Name
			}
		case ActionKindTool:
			spec, ok := FindToolAction(registry.Tools, action)
			if !ok {
				continue
			}
			action.ToolID = spec.ID
			if action.ID.Empty() {
				action.ID = spec.ID
			}
			if action.Name == "" {
				action.Name = spec.Name
			}
		case ActionKindSkill:
			spec, ok := FindSkillAction(registry.Skills, action)
			if !ok || strings.TrimSpace(action.Task) == "" {
				continue
			}
			action.SkillID = spec.ID
			if action.ID.Empty() {
				action.ID = spec.ID
			}
			if action.Name == "" {
				action.Name = spec.Name
			}
		default:
			continue
		}
		result.Actions = append(result.Actions, action)
		if maxActions > 0 && len(result.Actions) >= maxActions {
			break
		}
	}
	return result
}

func NormalizeAction(action Action) Action {
	action.Type = ActionKind(strings.ToLower(strings.TrimSpace(string(action.Type))))
	action.ID = id.ID(strings.TrimSpace(action.ID.String()))
	action.NodeID = id.ID(strings.TrimSpace(action.NodeID.String()))
	action.AgentID = id.ID(strings.TrimSpace(action.AgentID.String()))
	action.ToolID = id.ID(strings.TrimSpace(action.ToolID.String()))
	action.SkillID = id.ID(strings.TrimSpace(action.SkillID.String()))
	action.Name = strings.TrimSpace(action.Name)
	action.Task = strings.TrimSpace(action.Task)
	action.Reason = strings.TrimSpace(action.Reason)
	if action.Input == nil {
		action.Input = map[string]any{}
	}
	if action.Type == "" {
		switch {
		case !action.NodeID.Empty() || !action.AgentID.Empty():
			action.Type = ActionKindAgent
		case !action.ToolID.Empty():
			action.Type = ActionKindTool
		case !action.SkillID.Empty():
			action.Type = ActionKindSkill
		}
	}
	if action.ID.Empty() {
		switch action.Type {
		case ActionKindAgent:
			action.ID = action.AgentID
		case ActionKindTool:
			action.ID = action.ToolID
		case ActionKindSkill:
			action.ID = action.SkillID
		}
	}
	return action
}

func FindAgentAction(agents []AgentActionSpec, action Action) (AgentActionSpec, bool) {
	for _, spec := range agents {
		if (!action.NodeID.Empty() && spec.NodeID == action.NodeID) ||
			(!action.AgentID.Empty() && spec.AgentID == action.AgentID) ||
			(!action.ID.Empty() && spec.AgentID == action.ID) {
			return spec, true
		}
		if action.Name != "" && strings.EqualFold(spec.Name, action.Name) {
			return spec, true
		}
	}
	return AgentActionSpec{}, false
}

func FindToolAction(tools []tool.Spec, action Action) (tool.Spec, bool) {
	for _, spec := range tools {
		if (!action.ToolID.Empty() && spec.ID == action.ToolID) || (!action.ID.Empty() && spec.ID == action.ID) {
			return spec, true
		}
		if action.Name != "" && strings.EqualFold(spec.Name, action.Name) {
			return spec, true
		}
	}
	return tool.Spec{}, false
}

func FindSkillAction(skills []skill.Spec, action Action) (skill.Spec, bool) {
	for _, spec := range skills {
		if (!action.SkillID.Empty() && spec.ID == action.SkillID) || (!action.ID.Empty() && spec.ID == action.ID) {
			return spec, true
		}
		if action.Name != "" && strings.EqualFold(spec.Name, action.Name) {
			return spec, true
		}
	}
	return skill.Spec{}, false
}

func extractJSONObject(text string) string {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return trimmed
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end <= start {
		return ""
	}
	return trimmed[start : end+1]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolvedPromptAgentMentions(prompt string, agents []AgentActionSpec) []resolvedAgentMention {
	mentions := extractAgentMentionNames(prompt)
	if len(mentions) == 0 || len(agents) == 0 {
		return nil
	}

	result := make([]resolvedAgentMention, 0, len(mentions))
	seen := map[string]struct{}{}
	for _, mentionName := range mentions {
		agent, ok := findAgentByName(agents, mentionName)
		if !ok {
			continue
		}
		key := strings.ToLower(agent.Name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, resolvedAgentMention{
			MentionName: mentionName,
			Agent:       agent,
		})
	}
	return result
}

func extractAgentMentionNames(prompt string) []string {
	const marker = "@agent("
	names := []string{}
	remaining := prompt
	for {
		start := strings.Index(remaining, marker)
		if start < 0 {
			return names
		}
		afterMarker := remaining[start+len(marker):]
		end := strings.Index(afterMarker, ")")
		if end < 0 {
			return names
		}
		name := strings.TrimSpace(afterMarker[:end])
		if name != "" {
			names = append(names, name)
		}
		remaining = afterMarker[end+1:]
	}
}

func findAgentByName(agents []AgentActionSpec, name string) (AgentActionSpec, bool) {
	normalizedName := strings.TrimSpace(name)
	for _, agent := range agents {
		if strings.EqualFold(strings.TrimSpace(agent.Name), normalizedName) {
			return agent, true
		}
	}
	return AgentActionSpec{}, false
}
