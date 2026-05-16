package contextengine

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"flow-anything/internal/platform/contracts/model"
)

const (
	defaultMaxApproxTokens    = 32000
	defaultMaxToolResultChars = 12000
	defaultMaxHistoryMessages = 20
)

// Policy controls how much context can be sent to the model.
//
// The first version uses an approximate token budget so the runtime can make
// deterministic decisions without adding a tokenizer dependency. The policy is
// intentionally model-agnostic; model-specific windows can be wired in later.
type Policy struct {
	MaxHistoryMessages  int
	MaxApproxTokens     int
	MaxToolResultChars  int
	MaxMessageTextChars int
}

type Request struct {
	SystemPrompt string
	UserText     string
	History      []model.Message
	Policy       Policy
}

type Result struct {
	Messages []model.Message
	Report   Report
}

type Report struct {
	MessageCount         int           `json:"message_count"`
	EstimatedTokens      int           `json:"estimated_tokens"`
	HistoryMessageCount  int           `json:"history_message_count"`
	IncludedHistoryCount int           `json:"included_history_count"`
	DroppedHistoryCount  int           `json:"dropped_history_count"`
	TruncatedCount       int           `json:"truncated_count"`
	Blocks               []BlockReport `json:"blocks"`
}

type BlockReport struct {
	Key             string `json:"key"`
	Type            string `json:"type"`
	Label           string `json:"label"`
	MessageCount    int    `json:"message_count"`
	EstimatedTokens int    `json:"estimated_tokens"`
	Included        bool   `json:"included"`
	DroppedReason   string `json:"dropped_reason,omitempty"`
	TruncatedCount  int    `json:"truncated_count,omitempty"`
}

type Assembler struct {
	policy Policy
}

func NewAssembler(policy Policy) Assembler {
	return Assembler{policy: normalizePolicy(policy)}
}

func DefaultPolicy() Policy {
	return Policy{
		MaxHistoryMessages:  defaultMaxHistoryMessages,
		MaxApproxTokens:     defaultMaxApproxTokens,
		MaxToolResultChars:  defaultMaxToolResultChars,
		MaxMessageTextChars: defaultMaxToolResultChars,
	}
}

// Assemble builds the final LLM message list from structured context blocks.
// System prompt and current user input are mandatory; history is included from
// newest to oldest until the configured budget is exhausted.
func (a Assembler) Assemble(req Request) Result {
	policy := a.policy
	if !isZeroPolicy(req.Policy) {
		policy = normalizePolicy(req.Policy)
	}

	system := model.Message{Role: model.RoleSystem, Content: strings.TrimSpace(req.SystemPrompt)}
	user := model.Message{Role: model.RoleUser, Content: req.UserText}
	history := compactHistory(boundHistory(req.History, policy.MaxHistoryMessages), policy)

	fixedTokens := estimateMessagesTokens([]model.Message{system, user})
	availableHistoryTokens := policy.MaxApproxTokens - fixedTokens
	if availableHistoryTokens < 0 {
		availableHistoryTokens = 0
	}

	selectedHistory, droppedHistory := selectHistoryWithinBudget(history, availableHistoryTokens)
	messages := make([]model.Message, 0, 2+len(selectedHistory))
	messages = append(messages, system)
	messages = append(messages, selectedHistory...)
	messages = append(messages, user)

	report := Report{
		MessageCount:         len(messages),
		EstimatedTokens:      estimateMessagesTokens(messages),
		HistoryMessageCount:  len(history),
		IncludedHistoryCount: len(selectedHistory),
		DroppedHistoryCount:  droppedHistory,
		TruncatedCount:       countTruncated(history),
		Blocks: []BlockReport{
			blockReport("system_prompt", "system", "System Prompt", []model.Message{system}, true, "", 0),
			blockReport("conversation_history", "history", "Conversation History", selectedHistory, len(selectedHistory) > 0, droppedReason(droppedHistory), countTruncated(selectedHistory)),
			blockReport("current_user", "user", "Current User Request", []model.Message{user}, true, "", 0),
		},
	}

	return Result{Messages: messages, Report: report}
}

// CompactMessages applies the same context budget to an already assembled
// runtime message list. This is used after tool execution, where large tool
// results may have been appended after the initial conversation assembly.
func (a Assembler) CompactMessages(messages []model.Message) Result {
	policy := a.policy
	normalized := make([]model.Message, len(messages))
	copy(normalized, messages)
	normalized = compactHistory(normalized, policy)
	if len(normalized) <= 2 {
		report := reportForCompactedMessages(normalized, len(messages), 0)
		return Result{Messages: normalized, Report: report}
	}

	head := normalized[0]
	tailStart := runtimeTailStart(normalized)
	if anchor := latestUserBefore(normalized, tailStart); anchor > 0 {
		tailStart = anchor
	}
	tail := normalized[tailStart:]
	middle := normalized[1:tailStart]
	fixedTokens := estimateMessagesTokens(append([]model.Message{head}, tail...))
	budget := policy.MaxApproxTokens - fixedTokens
	if budget <= 0 {
		compacted := append([]model.Message{head}, tail...)
		report := reportForCompactedMessages(compacted, len(messages), len(middle))
		return Result{Messages: compacted, Report: report}
	}

	selectedMiddle, droppedMiddle := selectHistoryWithinBudget(middle, budget)
	compacted := make([]model.Message, 0, 1+len(selectedMiddle)+len(tail))
	compacted = append(compacted, head)
	compacted = append(compacted, selectedMiddle...)
	compacted = append(compacted, tail...)
	report := reportForCompactedMessages(compacted, len(messages), droppedMiddle)
	return Result{Messages: compacted, Report: report}
}

func normalizePolicy(policy Policy) Policy {
	defaults := DefaultPolicy()
	if policy.MaxHistoryMessages <= 0 {
		policy.MaxHistoryMessages = defaults.MaxHistoryMessages
	}
	if policy.MaxApproxTokens <= 0 {
		policy.MaxApproxTokens = defaults.MaxApproxTokens
	}
	if policy.MaxToolResultChars <= 0 {
		policy.MaxToolResultChars = defaults.MaxToolResultChars
	}
	if policy.MaxMessageTextChars <= 0 {
		policy.MaxMessageTextChars = defaults.MaxMessageTextChars
	}
	return policy
}

func isZeroPolicy(policy Policy) bool {
	return policy.MaxHistoryMessages == 0 &&
		policy.MaxApproxTokens == 0 &&
		policy.MaxToolResultChars == 0 &&
		policy.MaxMessageTextChars == 0
}

func boundHistory(history []model.Message, limit int) []model.Message {
	if limit > 0 && len(history) > limit {
		history = history[len(history)-limit:]
	}
	result := make([]model.Message, len(history))
	copy(result, history)
	return result
}

func compactHistory(history []model.Message, policy Policy) []model.Message {
	result := make([]model.Message, 0, len(history))
	for _, message := range history {
		message.Content = compactMessageContent(message, policy)
		result = append(result, message)
	}
	return result
}

func compactMessageContent(message model.Message, policy Policy) string {
	limit := policy.MaxMessageTextChars
	if message.Role == model.RoleTool {
		limit = policy.MaxToolResultChars
	}
	if limit <= 0 || utf8.RuneCountInString(message.Content) <= limit {
		return message.Content
	}
	return truncateRunes(message.Content, limit) + fmt.Sprintf("\n\n[Context truncated: original_chars=%d]", utf8.RuneCountInString(message.Content))
}

func selectHistoryWithinBudget(history []model.Message, budget int) ([]model.Message, int) {
	if len(history) == 0 {
		return nil, 0
	}
	if budget <= 0 {
		return nil, len(history)
	}

	selected := make([]model.Message, 0, len(history))
	used := 0
	for index := len(history) - 1; index >= 0; index-- {
		message := history[index]
		cost := estimateMessagesTokens([]model.Message{message})
		if used+cost > budget {
			break
		}
		selected = append(selected, message)
		used += cost
	}

	reverseMessages(selected)
	selected = dropLeadingToolMessages(selected)
	return selected, len(history) - len(selected)
}

func reportForCompactedMessages(messages []model.Message, originalCount int, droppedCount int) Report {
	return Report{
		MessageCount:         len(messages),
		EstimatedTokens:      estimateMessagesTokens(messages),
		HistoryMessageCount:  originalCount,
		IncludedHistoryCount: len(messages),
		DroppedHistoryCount:  droppedCount,
		TruncatedCount:       countTruncated(messages),
		Blocks: []BlockReport{
			blockReport("runtime_messages", "runtime", "Runtime Messages", messages, true, droppedReason(droppedCount), countTruncated(messages)),
		},
	}
}

func dropLeadingToolMessages(messages []model.Message) []model.Message {
	index := 0
	for index < len(messages) && messages[index].Role == model.RoleTool {
		index++
	}
	if index == 0 {
		return messages
	}
	result := make([]model.Message, len(messages)-index)
	copy(result, messages[index:])
	return result
}

func reverseMessages(messages []model.Message) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}

func runtimeTailStart(messages []model.Message) int {
	last := len(messages) - 1
	if messages[last].Role != model.RoleTool {
		return last
	}
	firstTrailingTool := last
	for firstTrailingTool > 0 && messages[firstTrailingTool-1].Role == model.RoleTool {
		firstTrailingTool--
	}
	if firstTrailingTool > 0 && messages[firstTrailingTool-1].Role == model.RoleAssistant {
		return firstTrailingTool - 1
	}
	return firstTrailingTool
}

func latestUserBefore(messages []model.Message, end int) int {
	if end > len(messages) {
		end = len(messages)
	}
	for index := end - 1; index > 0; index-- {
		if messages[index].Role == model.RoleUser {
			return index
		}
	}
	return -1
}

func blockReport(key, blockType, label string, messages []model.Message, included bool, droppedReason string, truncatedCount int) BlockReport {
	return BlockReport{
		Key:             key,
		Type:            blockType,
		Label:           label,
		MessageCount:    len(messages),
		EstimatedTokens: estimateMessagesTokens(messages),
		Included:        included,
		DroppedReason:   droppedReason,
		TruncatedCount:  truncatedCount,
	}
}

func droppedReason(count int) string {
	if count <= 0 {
		return ""
	}
	return fmt.Sprintf("%d older history messages omitted by context budget", count)
}

func countTruncated(messages []model.Message) int {
	count := 0
	for _, message := range messages {
		if strings.Contains(message.Content, "[Context truncated:") {
			count++
		}
	}
	return count
}

func estimateMessagesTokens(messages []model.Message) int {
	total := 0
	for _, message := range messages {
		total += approximateTokens(string(message.Role))
		total += approximateTokens(message.Content)
		for _, call := range message.ToolCalls {
			total += approximateTokens(call.ID)
			total += approximateTokens(call.Function.Name)
			for key, value := range call.Function.Arguments {
				total += approximateTokens(key)
				total += approximateTokens(fmt.Sprint(value))
			}
		}
		total += approximateTokens(message.ToolCallID)
	}
	return total
}

func approximateTokens(text string) int {
	runes := utf8.RuneCountInString(text)
	if runes == 0 {
		return 0
	}
	// A conservative approximation for mixed Chinese/English text. It is good
	// enough for deterministic budgeting until model-specific tokenizers are
	// introduced.
	return (runes + 2) / 3
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	var builder strings.Builder
	builder.Grow(limit)
	count := 0
	for _, r := range text {
		if count >= limit {
			break
		}
		builder.WriteRune(r)
		count++
	}
	return builder.String()
}
