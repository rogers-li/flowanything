package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

type ContextPhase string

const (
	ContextPhaseDirect        ContextPhase = "direct"
	ContextPhasePlanning      ContextPhase = "planning"
	ContextPhaseReActPlanning ContextPhase = "react_planning"
	ContextPhaseFinalAnswer   ContextPhase = "final_answer"
)

// MemoryProvider is the core-level extension point for short-term memory,
// long-term memory, user profile, or retrieval results. Storage and retrieval
// implementations live outside agentcore.
type MemoryProvider interface {
	Recall(ctx Context, req MemoryRecallRequest) ([]MemoryItem, error)
}

type MemoryRecallRequest struct {
	Agent       AgentSpec
	UserMessage string
	Context     map[string]any
	Limit       int
	TraceID     string
}

type MemoryItem struct {
	ID        string
	Type      string
	Content   string
	Score     float64
	Metadata  map[string]any
	Priority  int
	Estimated int
}

type ContextAssembler interface {
	Assemble(ctx Context, req ContextAssemblyRequest) (ContextAssembly, error)
}

type ContextAssemblyRequest struct {
	Agent         AgentSpec
	Phase         ContextPhase
	SystemPrompt  string
	UserMessage   string
	Conversation  []Message
	Context       map[string]any
	Memories      []MemoryItem
	Observations  []ActionResult
	Capabilities  []CapabilityDescriptor
	ExtraMessages []Message
}

type ContextAssembly struct {
	Messages []Message
	Report   ContextAssemblyReport
}

type ContextAssemblyReport struct {
	Phase                 ContextPhase         `json:"phase"`
	MessageCount          int                  `json:"message_count"`
	EstimatedTokens       int                  `json:"estimated_tokens"`
	IncludedHistoryCount  int                  `json:"included_history_count"`
	DroppedHistoryCount   int                  `json:"dropped_history_count"`
	IncludedMemoryCount   int                  `json:"included_memory_count"`
	DroppedMemoryCount    int                  `json:"dropped_memory_count"`
	IncludedContext       bool                 `json:"included_context"`
	IncludedObservations  bool                 `json:"included_observations"`
	TruncatedMessageCount int                  `json:"truncated_message_count"`
	Blocks                []ContextBlockReport `json:"blocks"`
}

type ContextBlockReport struct {
	Key             string `json:"key"`
	Type            string `json:"type"`
	Included        bool   `json:"included"`
	EstimatedTokens int    `json:"estimated_tokens"`
	DroppedCount    int    `json:"dropped_count,omitempty"`
	TruncatedCount  int    `json:"truncated_count,omitempty"`
}

type DefaultContextAssembler struct{}

func NewDefaultContextAssembler() DefaultContextAssembler {
	return DefaultContextAssembler{}
}

func (DefaultContextAssembler) Assemble(_ Context, req ContextAssemblyRequest) (ContextAssembly, error) {
	policy := normalizeAgentPolicy(req.Agent.Policy)
	system := Message{Role: "system", Content: strings.TrimSpace(req.SystemPrompt)}
	user := Message{Role: "user", Content: compactContent(strings.TrimSpace(req.UserMessage), policy.MaxMessageChars)}
	history := compactMessages(limitMessages(req.Conversation, policy.MaxHistoryMessages), policy.MaxMessageChars)
	memoryMessage := buildMemoryMessage(selectMemoryItems(req.Memories, policy.MaxMemoryItems))
	memoryMessage.Content = compactContent(memoryMessage.Content, policy.MaxMessageChars)
	contextMessage := buildRuntimeContextMessage(req.Context)
	contextMessage.Content = compactContent(contextMessage.Content, policy.MaxMessageChars)
	observationMessage := buildObservationMessage(req.Observations)
	observationMessage.Content = compactContent(observationMessage.Content, policy.MaxMessageChars)

	fixed := []Message{system}
	if memoryMessage.Content != "" {
		fixed = append(fixed, memoryMessage)
	}
	if contextMessage.Content != "" {
		fixed = append(fixed, contextMessage)
	}
	if observationMessage.Content != "" {
		fixed = append(fixed, observationMessage)
	}
	fixed = append(fixed, compactMessages(req.ExtraMessages, policy.MaxMessageChars)...)
	fixed = append(fixed, user)

	fixedTokens := estimateMessagesTokens(fixed)
	historyBudget := policy.MaxContextTokens - fixedTokens
	if historyBudget < 0 {
		historyBudget = 0
	}
	selectedHistory, droppedHistory := selectMessagesWithinBudget(history, historyBudget)

	messages := make([]Message, 0, len(fixed)+len(selectedHistory))
	messages = append(messages, system)
	if memoryMessage.Content != "" {
		messages = append(messages, memoryMessage)
	}
	if contextMessage.Content != "" {
		messages = append(messages, contextMessage)
	}
	messages = append(messages, selectedHistory...)
	if observationMessage.Content != "" {
		messages = append(messages, observationMessage)
	}
	messages = append(messages, compactMessages(req.ExtraMessages, policy.MaxMessageChars)...)
	messages = append(messages, user)

	selectedMemoryCount := len(selectMemoryItems(req.Memories, policy.MaxMemoryItems))
	report := ContextAssemblyReport{
		Phase:                 req.Phase,
		MessageCount:          len(messages),
		EstimatedTokens:       estimateMessagesTokens(messages),
		IncludedHistoryCount:  len(selectedHistory),
		DroppedHistoryCount:   droppedHistory,
		IncludedMemoryCount:   selectedMemoryCount,
		DroppedMemoryCount:    maxInt(0, len(req.Memories)-selectedMemoryCount),
		IncludedContext:       contextMessage.Content != "",
		IncludedObservations:  observationMessage.Content != "",
		TruncatedMessageCount: countTruncatedMessages(messages),
		Blocks: []ContextBlockReport{
			blockReport("system_prompt", "system", []Message{system}, true, 0),
			blockReport("memory", "memory", []Message{memoryMessage}, memoryMessage.Content != "", maxInt(0, len(req.Memories)-selectedMemoryCount)),
			blockReport("runtime_context", "context", []Message{contextMessage}, contextMessage.Content != "", 0),
			blockReport("conversation_history", "history", selectedHistory, len(selectedHistory) > 0, droppedHistory),
			blockReport("observations", "observations", []Message{observationMessage}, observationMessage.Content != "", 0),
			blockReport("current_user", "user", []Message{user}, true, 0),
		},
	}
	return ContextAssembly{Messages: messages, Report: report}, nil
}

func recallMemories(ctx Context, provider MemoryProvider, req AgentRunRequest) ([]MemoryItem, error) {
	if provider == nil {
		return nil, nil
	}
	policy := normalizeAgentPolicy(req.Agent.Policy)
	return provider.Recall(ctx, MemoryRecallRequest{
		Agent:       req.Agent,
		UserMessage: req.UserMessage,
		Context:     req.Context,
		Limit:       policy.MaxMemoryItems,
		TraceID:     req.TraceID,
	})
}

func buildMemoryMessage(items []MemoryItem) Message {
	if len(items) == 0 {
		return Message{}
	}
	var builder strings.Builder
	builder.WriteString("Relevant memory:\n")
	for _, item := range items {
		label := item.Type
		if label == "" {
			label = "memory"
		}
		builder.WriteString(fmt.Sprintf("- [%s] %s", label, item.Content))
		if item.Score > 0 {
			builder.WriteString(fmt.Sprintf(" (score=%.3f)", item.Score))
		}
		builder.WriteString("\n")
	}
	return Message{Role: "assistant", Content: strings.TrimSpace(builder.String())}
}

func buildRuntimeContextMessage(context map[string]any) Message {
	if len(context) == 0 {
		return Message{}
	}
	data, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return Message{}
	}
	return Message{Role: "assistant", Content: "Runtime context:\n" + string(data)}
}

func buildObservationMessage(observations []ActionResult) Message {
	if len(observations) == 0 {
		return Message{}
	}
	data, _ := json.MarshalIndent(observations, "", "  ")
	return Message{Role: "assistant", Content: "Observations:\n" + string(data)}
}

func selectMemoryItems(items []MemoryItem, limit int) []MemoryItem {
	if limit <= 0 || len(items) <= limit {
		out := make([]MemoryItem, len(items))
		copy(out, items)
		return out
	}
	out := make([]MemoryItem, limit)
	copy(out, items[:limit])
	return out
}

func limitMessages(messages []Message, limit int) []Message {
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	out := make([]Message, len(messages))
	copy(out, messages)
	return out
}

func compactMessages(messages []Message, maxChars int) []Message {
	out := make([]Message, 0, len(messages))
	for _, message := range messages {
		message.Content = compactContent(message.Content, maxChars)
		out = append(out, message)
	}
	return out
}

func compactContent(content string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(content) <= maxChars {
		return content
	}
	return truncateRunes(content, maxChars) + fmt.Sprintf("\n\n[Context truncated: original_chars=%d]", utf8.RuneCountInString(content))
}

func selectMessagesWithinBudget(messages []Message, budget int) ([]Message, int) {
	if len(messages) == 0 {
		return nil, 0
	}
	if budget <= 0 {
		return nil, len(messages)
	}
	selected := make([]Message, 0, len(messages))
	used := 0
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		cost := estimateMessagesTokens([]Message{message})
		if used+cost > budget {
			break
		}
		selected = append(selected, message)
		used += cost
	}
	reverseMessages(selected)
	return selected, len(messages) - len(selected)
}

func estimateMessagesTokens(messages []Message) int {
	total := 0
	for _, message := range messages {
		total += estimateTokens(message.Content) + 4
	}
	return total
}

func estimateTokens(text string) int {
	runes := utf8.RuneCountInString(text)
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func countTruncatedMessages(messages []Message) int {
	count := 0
	for _, message := range messages {
		if strings.Contains(message.Content, "[Context truncated:") {
			count++
		}
	}
	return count
}

func blockReport(key, blockType string, messages []Message, included bool, droppedCount int) ContextBlockReport {
	return ContextBlockReport{
		Key:             key,
		Type:            blockType,
		Included:        included,
		EstimatedTokens: estimateMessagesTokens(messages),
		DroppedCount:    droppedCount,
		TruncatedCount:  countTruncatedMessages(messages),
	}
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func reverseMessages(messages []Message) {
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
