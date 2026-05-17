package contextengine

import (
	"strings"
	"testing"

	"flow-anything/internal/platform/contracts/model"
)

func TestAssemblerKeepsSystemHistoryAndCurrentUserOrder(t *testing.T) {
	t.Parallel()

	result := NewAssembler(DefaultPolicy()).Assemble(Request{
		SystemPrompt: "system",
		History: []model.Message{
			{Role: model.RoleUser, Content: "hello"},
			{Role: model.RoleAssistant, Content: "hi"},
		},
		UserText: "continue",
	})

	if len(result.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %#v", result.Messages)
	}
	if result.Messages[0].Role != model.RoleSystem || result.Messages[0].Content != "system" {
		t.Fatalf("unexpected system message %#v", result.Messages[0])
	}
	if result.Messages[1].Content != "hello" || result.Messages[2].Content != "hi" || result.Messages[3].Content != "continue" {
		t.Fatalf("messages are not in expected order: %#v", result.Messages)
	}
	if result.Report.IncludedHistoryCount != 2 || result.Report.DroppedHistoryCount != 0 {
		t.Fatalf("unexpected report %#v", result.Report)
	}
}

func TestAssemblerDropsOldHistoryWhenBudgetIsExceeded(t *testing.T) {
	t.Parallel()

	result := NewAssembler(Policy{
		MaxHistoryMessages:  10,
		MaxApproxTokens:     12,
		MaxToolResultChars:  1000,
		MaxMessageTextChars: 1000,
	}).Assemble(Request{
		SystemPrompt: "system",
		History: []model.Message{
			{Role: model.RoleUser, Content: strings.Repeat("old ", 20)},
			{Role: model.RoleAssistant, Content: "recent"},
		},
		UserText: "now",
	})

	if result.Report.DroppedHistoryCount == 0 {
		t.Fatalf("expected older history to be dropped, report=%#v", result.Report)
	}
	if got := result.Messages[len(result.Messages)-2].Content; got != "recent" {
		t.Fatalf("expected recent history to be retained, got %q in %#v", got, result.Messages)
	}
}

func TestAssemblerTruncatesLargeToolResults(t *testing.T) {
	t.Parallel()

	result := NewAssembler(Policy{
		MaxHistoryMessages:  10,
		MaxApproxTokens:     1000,
		MaxToolResultChars:  8,
		MaxMessageTextChars: 1000,
	}).Assemble(Request{
		SystemPrompt: "system",
		History: []model.Message{
			{
				Role: model.RoleAssistant,
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Function: model.ToolCallFunction{Name: "query"}},
				},
			},
			{Role: model.RoleTool, ToolCallID: "call_1", Content: "0123456789abcdef"},
		},
		UserText: "continue",
	})

	if len(result.Messages) != 4 {
		t.Fatalf("expected tool history to remain with system/current user, got %#v", result.Messages)
	}
	if !strings.Contains(result.Messages[2].Content, "[Context truncated:") {
		t.Fatalf("expected truncated marker, got %q", result.Messages[2].Content)
	}
	if result.Report.TruncatedCount != 1 {
		t.Fatalf("expected report truncation count, got %#v", result.Report)
	}
}

func TestAssemblerDropsHistoryWhenFixedMessagesExceedBudget(t *testing.T) {
	t.Parallel()

	result := NewAssembler(Policy{
		MaxHistoryMessages:  10,
		MaxApproxTokens:     1,
		MaxToolResultChars:  1000,
		MaxMessageTextChars: 1000,
	}).Assemble(Request{
		SystemPrompt: "system prompt already exceeds budget",
		History: []model.Message{
			{Role: model.RoleUser, Content: "history"},
		},
		UserText: "current user",
	})

	if len(result.Messages) != 2 {
		t.Fatalf("expected only system and current user, got %#v", result.Messages)
	}
	if result.Report.DroppedHistoryCount != 1 {
		t.Fatalf("expected history to be dropped, got %#v", result.Report)
	}
}

func TestCompactMessagesTruncatesToolResultBeforeFinalAnswer(t *testing.T) {
	t.Parallel()

	result := NewAssembler(Policy{
		MaxHistoryMessages:  10,
		MaxApproxTokens:     1000,
		MaxToolResultChars:  8,
		MaxMessageTextChars: 1000,
	}).CompactMessages([]model.Message{
		{Role: model.RoleSystem, Content: "system"},
		{Role: model.RoleUser, Content: "question"},
		{
			Role: model.RoleAssistant,
			ToolCalls: []model.ToolCall{
				{ID: "call_1", Function: model.ToolCallFunction{Name: "query"}},
			},
		},
		{Role: model.RoleTool, ToolCallID: "call_1", Content: "0123456789abcdef"},
	})

	if !strings.Contains(result.Messages[len(result.Messages)-1].Content, "[Context truncated:") {
		t.Fatalf("expected runtime tool result to be compacted, got %#v", result.Messages)
	}
}
