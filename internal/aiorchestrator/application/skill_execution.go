package application

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	orchestration "flow-anything/internal/agentorchestration"
	"flow-anything/internal/aiorchestrator/domain"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
)

const maxSkillToolIterations = 12

type skillExecutionBinding struct {
	Spec  skill.Spec
	Tools []tool.Spec
}

func skillExecutionFromPayload(payload map[string]any, config domain.AgentConfig) (bool, skill.Spec, map[string]any) {
	if payload == nil {
		return false, skill.Spec{}, nil
	}
	rawSkillID, _ := payload["skill_id"].(string)
	if rawSkillID == "" {
		return false, skill.Spec{}, nil
	}

	for _, spec := range config.Skills {
		if spec.ID == id.ID(rawSkillID) {
			return true, spec, mergeSkillInputDefaults(skillDefaultInput(spec), skillInputFromPayload(payload))
		}
	}
	return false, skill.Spec{}, nil
}

func skillInputFromPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	for _, key := range []string{"skill_input", "input"} {
		if input, ok := payload[key].(map[string]any); ok {
			return input
		}
	}
	return nil
}

func toolsForSkill(tools []tool.Spec, spec skill.Spec) []tool.Spec {
	if len(spec.ToolIDs) == 0 || len(tools) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(spec.ToolIDs))
	for _, toolID := range spec.ToolIDs {
		allowed[toolID.String()] = struct{}{}
	}
	result := make([]tool.Spec, 0, len(spec.ToolIDs))
	for _, candidate := range tools {
		if _, ok := allowed[candidate.ID.String()]; ok {
			result = append(result, candidate)
		}
	}
	return result
}

func toModelSkillTools(skills []skill.Spec) []model.ToolDefinition {
	result := make([]model.ToolDefinition, 0, len(skills))
	functionNames := modelFunctionNameMapForSkills(skills)
	for _, spec := range skills {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		result = append(result, model.ToolDefinition{
			Type: "function",
			Function: model.ToolFunction{
				Name:        functionNames[spec.ID.String()],
				Description: modelSkillDescription(spec),
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"task": map[string]any{
							"type":        "string",
							"description": "Focused task for this skill. Keep it self-contained.",
						},
						"input": map[string]any{
							"type":        "object",
							"description": "Structured input for the skill. You may also pass skill-specific fields at the top level.",
						},
					},
				},
			},
		})
	}
	return result
}

func modelSkillDescription(spec skill.Spec) string {
	var builder strings.Builder
	builder.WriteString("Execute Skill: ")
	builder.WriteString(firstNonEmpty(spec.Description, spec.SystemPrompt, spec.Name))
	if strings.TrimSpace(spec.Name) != "" {
		builder.WriteString("\nSkill display name: ")
		builder.WriteString(strings.TrimSpace(spec.Name))
	}
	if !spec.ID.Empty() {
		builder.WriteString("\nSkill ID: ")
		builder.WriteString(spec.ID.String())
	}
	if len(spec.ToolIDs) > 0 {
		toolIDs, _ := json.Marshal(idsToStrings(spec.ToolIDs))
		builder.WriteString("\nSkill owns tools: ")
		builder.Write(toolIDs)
	}
	if strings.TrimSpace(spec.OutputFormat) != "" {
		builder.WriteString("\nExpected output: ")
		builder.WriteString(strings.TrimSpace(spec.OutputFormat))
	}
	return builder.String()
}

func mapSkillExecutionBindings(skills []skill.Spec, tools []tool.Spec) map[string]skillExecutionBinding {
	result := make(map[string]skillExecutionBinding, len(skills))
	functionNames := modelFunctionNameMapForSkills(skills)
	for _, spec := range skills {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		result[functionNames[spec.ID.String()]] = skillExecutionBinding{
			Spec:  spec,
			Tools: toolsForSkill(tools, spec),
		}
	}
	return result
}

func (s *Service) executeSkillToolCall(ctx context.Context, evt event.Event, binding skillExecutionBinding, call model.ToolCall, parent toolExecutionContext) (string, error) {
	task, input := skillTaskAndInput(binding.Spec, call.Function.Arguments)
	startedAt := time.Now().UTC()
	s.logger.Info("skill call started",
		"event_id", evt.ID.String(),
		"trace_id", evt.TraceID,
		"agent_id", evt.AgentID.String(),
		"tool_call_id", call.ID,
		"skill_id", binding.Spec.ID.String(),
		"skill_name", binding.Spec.Name,
		"tool_count", len(binding.Tools),
	)

	runResult, err := s.executePromptToolRun(ctx, evt, promptToolExecution{
		SystemPrompt: orchestration.BuildSkillActionSystemPrompt(binding.Spec, orchestration.Action{
			Type:    orchestration.ActionKindSkill,
			ID:      binding.Spec.ID,
			SkillID: binding.Spec.ID,
			Name:    binding.Spec.Name,
			Task:    task,
			Input:   input,
		}),
		UserText:        task,
		ModelName:       parent.ModelName,
		ModelOptions:    parent.ModelOptions,
		Tools:           toModelTools(binding.Tools),
		ToolByName:      mapToolsByName(binding.Tools),
		SkillRefsByTool: skillRefsByToolID([]skill.Spec{binding.Spec}),
		DefaultToolArgsByName: defaultToolArgsForSkill(
			binding.Spec,
			input,
		),
		MaxToolIterations: skillMaxToolIterations(binding.Spec, 1),
	})
	reply := runResult.Reply
	finishedAt := time.Now().UTC()
	status := domain.TraceStepStatusSucceeded
	if err != nil {
		status = domain.TraceStepStatusFailed
	}
	s.appendTraceStep(ctx, evt, domain.TraceStepSkill, binding.Spec.Name, status, startedAt, finishedAt, map[string]any{
		"skill_id":       binding.Spec.ID.String(),
		"execution_mode": "skill_tool",
		"tool_call_id":   call.ID,
		"tool_count":     len(binding.Tools),
		"task":           task,
		"input":          sanitizeValue(input),
		"error":          errorText(err),
	})
	if err != nil {
		s.logger.Error("skill call failed",
			"event_id", evt.ID.String(),
			"trace_id", evt.TraceID,
			"agent_id", evt.AgentID.String(),
			"tool_call_id", call.ID,
			"skill_id", binding.Spec.ID.String(),
			"skill_name", binding.Spec.Name,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return "", err
	}

	s.logger.Info("skill call completed",
		"event_id", evt.ID.String(),
		"trace_id", evt.TraceID,
		"agent_id", evt.AgentID.String(),
		"tool_call_id", call.ID,
		"skill_id", binding.Spec.ID.String(),
		"skill_name", binding.Spec.Name,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
	payload, err := json.Marshal(map[string]any{
		"success":    true,
		"skill_id":   binding.Spec.ID.String(),
		"skill_name": binding.Spec.Name,
		"text":       reply,
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func skillTaskAndInput(spec skill.Spec, args map[string]any) (string, map[string]any) {
	task, _ := args["task"].(string)
	input, _ := args["input"].(map[string]any)
	if input == nil {
		input = make(map[string]any, len(args))
		for key, value := range args {
			if key == "task" {
				continue
			}
			input[key] = value
		}
	}
	if strings.TrimSpace(task) == "" {
		if value, ok := input["task"].(string); ok {
			task = value
		}
	}
	if strings.TrimSpace(task) == "" {
		if value, ok := input["user_request"].(string); ok {
			task = value
		}
	}
	if strings.TrimSpace(task) == "" {
		task = "Execute skill " + spec.Name
	}
	return strings.TrimSpace(task), mergeSkillInputDefaults(skillDefaultInput(spec), input)
}

func skillDefaultInput(spec skill.Spec) map[string]any {
	if !isFeishuDocWriteSkill(spec) {
		return nil
	}
	return feishuFolderTokenDefaultArgs()
}

func defaultToolArgsForSkill(spec skill.Spec, input map[string]any) map[string]map[string]any {
	if !isFeishuDocWriteSkill(spec) {
		return nil
	}
	folderToken, _ := input["folder_token"].(string)
	folderToken = strings.TrimSpace(folderToken)
	if folderToken == "" {
		return nil
	}
	return map[string]map[string]any{
		"feishu_create_document": {
			"folder_token": folderToken,
		},
	}
}

func defaultToolArgsForSpec(spec tool.Spec) map[string]any {
	if spec.Name != "feishu_create_document" && spec.Name != "feishu_upload_markdown" && spec.Binding.WorkflowID.String() != "wf_feishu_upload_markdown" {
		return nil
	}
	return feishuFolderTokenDefaultArgs()
}

func mergeSkillInputDefaults(defaults map[string]any, input map[string]any) map[string]any {
	if len(defaults) == 0 {
		return input
	}
	merged := make(map[string]any, len(defaults)+len(input))
	for key, value := range defaults {
		merged[key] = value
	}
	for key, value := range input {
		merged[key] = value
	}
	return merged
}

func isFeishuDocWriteSkill(spec skill.Spec) bool {
	return spec.ID.String() == "skill_feishu_doc_write" || spec.Name == "feishu_doc_write"
}

func feishuFolderTokenDefaultArgs() map[string]any {
	folderToken := strings.TrimSpace(os.Getenv("FEISHU_DOC_FOLDER_TOKEN"))
	if folderToken == "" {
		return nil
	}
	return map[string]any{
		"folder_token": folderToken,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func skillMaxToolIterations(spec skill.Spec, fallback int) int {
	limit := spec.ExecutionPolicy.MaxToolCalls
	if limit <= 0 {
		limit = fallback
	}
	if limit <= 0 {
		limit = 1
	}
	if limit > maxSkillToolIterations {
		return maxSkillToolIterations
	}
	return limit
}

func executionModeName(skillMode bool) string {
	if skillMode {
		return "skill"
	}
	return "agent"
}

func (s *Service) recordSkillExecutionTrace(ctx context.Context, evt event.Event, spec skill.Spec, tools []model.ToolDefinition, maxToolIterations int) {
	now := time.Now().UTC()
	s.appendTraceStep(ctx, evt, domain.TraceStepSkill, spec.Name, domain.TraceStepStatusSucceeded, now, now, map[string]any{
		"skill_id":            spec.ID.String(),
		"execution_mode":      "skill",
		"tool_count":          len(tools),
		"max_tool_iterations": maxToolIterations,
		"policy_version":      spec.PolicyVersion,
	})
}
