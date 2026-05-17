package infrastructure

import (
	"context"
	"strings"
	"testing"

	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestMockProviderReturnsToolCallThenFinalResponse(t *testing.T) {
	t.Parallel()

	provider := NewMockProvider()
	firstResp, err := provider.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "帮我查订单 o_123"},
		},
		Tools: []model.ToolDefinition{
			{
				Type: "function",
				Function: model.ToolFunction{
					Name: "query_order",
				},
			},
		},
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("Chat() first call error = %v", err)
	}
	if firstResp.FinishReason != "tool_calls" {
		t.Fatalf("expected tool_calls, got %q", firstResp.FinishReason)
	}
	if len(firstResp.Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(firstResp.Message.ToolCalls))
	}
	toolCall := firstResp.Message.ToolCalls[0]
	if toolCall.Function.Name != "query_order" {
		t.Fatalf("expected query_order tool, got %q", toolCall.Function.Name)
	}
	if toolCall.Function.Arguments["order_id"] != "o_123" {
		t.Fatalf("unexpected arguments %#v", toolCall.Function.Arguments)
	}

	secondResp, err := provider.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "帮我查订单 o_123"},
			firstResp.Message,
			{Role: model.RoleTool, Content: `{"success":true,"data":{"status":"paid"}}`, ToolCallID: toolCall.ID},
		},
	})
	if err != nil {
		t.Fatalf("Chat() second call error = %v", err)
	}
	if !strings.Contains(secondResp.Message.Content, "工具执行完成") {
		t.Fatalf("unexpected final response %q", secondResp.Message.Content)
	}
}

func TestMockProviderExtractsWeatherCity(t *testing.T) {
	t.Parallel()

	provider := NewMockProvider()
	resp, err := provider.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "帮我查询深圳天气"},
		},
		Tools: []model.ToolDefinition{
			{
				Type: "function",
				Function: model.ToolFunction{
					Name: "query_weather",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"city": map[string]any{"type": "string"},
						},
						"required": []any{"city"},
					},
				},
			},
		},
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(resp.Message.ToolCalls))
	}
	args := resp.Message.ToolCalls[0].Function.Arguments
	if args["city"] != "深圳" {
		t.Fatalf("expected weather city 深圳, got %#v", args)
	}
}

func TestMockProviderStartsNewToolCallWhenLatestMessageIsUser(t *testing.T) {
	t.Parallel()

	provider := NewMockProvider()
	resp, err := provider.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "帮我查询深圳天气"},
			{Role: model.RoleAssistant, ToolCalls: []model.ToolCall{{ID: "call_1", Type: "function"}}},
			{Role: model.RoleTool, Content: `{"success":true,"data":{"city":"深圳"}}`, ToolCallID: "call_1"},
			{Role: model.RoleAssistant, Content: "深圳天气已查到"},
			{Role: model.RoleUser, Content: "那北京呢？"},
		},
		Tools: []model.ToolDefinition{
			{
				Type: "function",
				Function: model.ToolFunction{
					Name: "query_weather",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"city": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.FinishReason != "tool_calls" {
		t.Fatalf("expected a new tool call, got finish reason %q and content %q", resp.FinishReason, resp.Message.Content)
	}
	args := resp.Message.ToolCalls[0].Function.Arguments
	if args["city"] != "北京" {
		t.Fatalf("expected weather city 北京, got %#v", args)
	}
}
