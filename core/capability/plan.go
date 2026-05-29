package capability

import (
	"encoding/json"
	"fmt"
	"strings"

	"flow-anything/core/jsonutil"
)

type PlannedAction struct {
	Kind   Kind           `json:"type"`
	ID     string         `json:"id"`
	Task   string         `json:"task"`
	Input  map[string]any `json:"input,omitempty"`
	Reason string         `json:"reason"`
}

type ActionPlan struct {
	Actions               []PlannedAction `json:"actions"`
	FinalAnswerIfNoAction string          `json:"final_answer_if_no_action,omitempty"`
}

func ParseActionPlan(content string) (ActionPlan, error) {
	candidates := jsonutil.ObjectCandidates(stripMarkdownFence(content))
	if len(candidates) == 0 {
		candidates = []string{strings.TrimSpace(stripMarkdownFence(content))}
	}
	var firstErr error
	for _, candidate := range candidates {
		plan, ok, err := decodeActionPlan(candidate)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if ok {
			return plan, nil
		}
	}
	if firstErr != nil {
		return ActionPlan{}, firstErr
	}
	return ActionPlan{}, fmt.Errorf("action plan JSON object not found")
}

func decodeActionPlan(content string) (ActionPlan, bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &fields); err != nil {
		return ActionPlan{}, false, err
	}
	_, hasActions := fields["actions"]
	_, hasFinalAnswer := fields["final_answer_if_no_action"]
	if !hasActions && !hasFinalAnswer {
		return ActionPlan{}, false, nil
	}
	var wire struct {
		Actions               []PlannedAction `json:"actions"`
		FinalAnswerIfNoAction json.RawMessage `json:"final_answer_if_no_action,omitempty"`
	}
	if err := json.Unmarshal([]byte(content), &wire); err != nil {
		return ActionPlan{}, false, err
	}
	finalAnswer, err := normalizeFinalAnswer(wire.FinalAnswerIfNoAction)
	if err != nil {
		return ActionPlan{}, false, err
	}
	return ActionPlan{
		Actions:               wire.Actions,
		FinalAnswerIfNoAction: finalAnswer,
	}, true, nil
}

func normalizeFinalAnswer(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("final_answer_if_no_action is not valid JSON")
	}
	return strings.TrimSpace(string(raw)), nil
}

func stripMarkdownFence(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}
