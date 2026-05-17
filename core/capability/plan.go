package capability

import (
	"encoding/json"
	"strings"
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
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var plan ActionPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return ActionPlan{}, err
	}
	return plan, nil
}
