package event

import (
	"time"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Type string

const (
	TypeUserMessageCommitted   Type = "user_message_committed"
	TypeUserUtteranceCommitted Type = "user_utterance_committed"
	TypeToolResultReceived     Type = "tool_result_received"
	TypeToolFailed             Type = "tool_failed"
)

type Channel string

const (
	ChannelText  Channel = "text"
	ChannelVoice Channel = "voice"
)

type Event struct {
	ID         id.ID          `json:"id"`
	TenantID   tenant.ID      `json:"tenant_id"`
	TraceID    string         `json:"trace_id"`
	UserID     string         `json:"user_id"`
	SessionID  id.ID          `json:"session_id"`
	TaskID     id.ID          `json:"task_id,omitempty"`
	AgentID    id.ID          `json:"agent_id"`
	Type       Type           `json:"type"`
	Channel    Channel        `json:"channel"`
	Payload    map[string]any `json:"payload,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
}

type ActionType string

const (
	ActionSpeak           ActionType = "speak"
	ActionDisplayText     ActionType = "display_text"
	ActionAskQuestion     ActionType = "ask_question"
	ActionAskConfirmation ActionType = "ask_confirmation"
	ActionCallTool        ActionType = "call_tool"
	ActionWait            ActionType = "wait"
	ActionEndTurn         ActionType = "end_turn"
)

type Action struct {
	Type       ActionType   `json:"type"`
	Text       string       `json:"text,omitempty"`
	ToolCall   *tool.Call   `json:"tool_call,omitempty"`
	ToolResult *tool.Result `json:"tool_result,omitempty"`
}

type Response struct {
	EventID id.ID    `json:"event_id"`
	TraceID string   `json:"trace_id"`
	Actions []Action `json:"actions"`
}
