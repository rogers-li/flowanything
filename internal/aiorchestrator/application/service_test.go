package application

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"flow-anything/internal/aiorchestrator/domain"
	"flow-anything/internal/aiorchestrator/infrastructure"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHandleEventReturnsSpeakAction(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := New(logger, nil, nil, nil)

	resp, err := service.HandleEvent(context.Background(), event.Event{
		Type: event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text": "hello",
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if len(resp.Actions) == 0 {
		t.Fatal("expected at least one action")
	}
	if resp.Actions[0].Type != event.ActionSpeak {
		t.Fatalf("expected first action %q, got %q", event.ActionSpeak, resp.Actions[0].Type)
	}
}

func TestHandleEventExecutesToolWhenPayloadHasToolID(t *testing.T) {
	t.Parallel()

	toolID := id.ID("tool_query_order")
	runtime := &fakeToolRuntime{
		result: tool.Result{
			ToolID:     toolID,
			Success:    true,
			Data:       map[string]any{"order_id": "o_123"},
			FinishedAt: time.Now().UTC(),
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), runtime, nil, nil)

	resp, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenant.ID("tenant_1"),
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text":    "帮我查订单",
			"tool_id": toolID.String(),
			"tool_args": map[string]any{
				"order_id": "o_123",
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if runtime.call.ToolID != toolID {
		t.Fatalf("expected tool id %q, got %q", toolID, runtime.call.ToolID)
	}
	if len(resp.Actions) == 0 || resp.Actions[0].ToolResult == nil {
		t.Fatal("expected first action to include tool result")
	}
	if !resp.Actions[0].ToolResult.Success {
		t.Fatal("expected successful tool result")
	}
}

func TestHandleEventPassesConfirmedFlagToExplicitTool(t *testing.T) {
	t.Parallel()

	toolID := id.ID("tool_refund_order")
	runtime := &fakeToolRuntime{
		result: tool.Result{
			ToolID:     toolID,
			Success:    true,
			FinishedAt: time.Now().UTC(),
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), runtime, nil, nil)

	_, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenant.ID("tenant_1"),
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"tool_id":   toolID.String(),
			"tool_args": map[string]any{"order_id": "o_123"},
			"confirmed": true,
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if !runtime.call.Confirmed {
		t.Fatal("expected confirmed flag to be passed to tool runtime")
	}
}

func TestHandleEventUsesModelClientForPlainMessage(t *testing.T) {
	t.Parallel()

	modelClient := &fakeModelClient{
		response: model.ChatResponse{
			Message: model.Message{
				Role:    model.RoleAssistant,
				Content: "模型回复",
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, modelClient, nil)

	resp, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenant.ID("tenant_1"),
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if len(modelClient.requests) == 0 || len(modelClient.requests[0].Messages) == 0 {
		t.Fatal("expected model request messages")
	}
	if resp.Actions[0].Text != "模型回复" {
		t.Fatalf("expected model reply, got %q", resp.Actions[0].Text)
	}
}

func TestHandleEventUsesConfiguredSystemPrompt(t *testing.T) {
	t.Parallel()

	modelClient := &fakeModelClient{
		response: model.ChatResponse{
			Message: model.Message{
				Role:    model.RoleAssistant,
				Content: "自定义提示词回复",
			},
		},
	}
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		modelClient,
		nil,
		WithDefaultSystemPrompt("你是企业客服助手。"),
	)

	_, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenant.ID("tenant_1"),
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if got := modelClient.requests[0].Messages[0].Content; got != "你是企业客服助手。" {
		t.Fatalf("expected configured system prompt, got %q", got)
	}
}

func TestHandleEventUsesAuthoredAgentPromptWithoutPlatformWrapper(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_search")
	modelClient := &fakeModelClient{
		response: model.ChatResponse{
			Message: model.Message{
				Role:    model.RoleAssistant,
				Content: "搜索完成",
			},
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{
				ID:           agentID,
				TenantID:     tenantID,
				Name:         "web_search",
				Description:  "search everything",
				SystemPrompt: "## Role\n你是一个知识搜索专家。",
			},
		},
	}
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		modelClient,
		configLoader,
		WithDefaultSystemPrompt("默认平台提示词"),
	)

	_, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload:  map[string]any{"text": "搜索 AI 新闻"},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if got := modelClient.requests[0].Messages[0].Content; got != "## Role\n你是一个知识搜索专家。" {
		t.Fatalf("expected authored agent prompt only, got %q", got)
	}
}

func TestHandleEventAppendsRuntimeSystemPromptWithoutPollutingUserMessage(t *testing.T) {
	t.Parallel()

	modelClient := &fakeModelClient{
		response: model.ChatResponse{
			Message: model.Message{
				Role:    model.RoleAssistant,
				Content: "规划完成",
			},
		},
	}
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		modelClient,
		nil,
		WithDefaultSystemPrompt("基础系统提示词"),
	)

	_, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenant.ID("tenant_1"),
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text":                        "真实用户请求",
			runtimeSystemPromptPayloadKey: "只返回 JSON",
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	messages := modelClient.requests[0].Messages
	if !strings.Contains(messages[0].Content, "基础系统提示词") || !strings.Contains(messages[0].Content, "只返回 JSON") {
		t.Fatalf("system prompt did not include runtime instructions: %q", messages[0].Content)
	}
	if messages[1].Role != model.RoleUser || messages[1].Content != "真实用户请求" {
		t.Fatalf("runtime instructions leaked into user message: %#v", messages[1])
	}
}

func TestHandleEventCanDisableRuntimeToolCalling(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	toolID := id.ID("tool_search")
	modelClient := &fakeModelClient{
		response: model.ChatResponse{
			Message: model.Message{
				Role:    model.RoleAssistant,
				Content: `{"actions":[],"final_answer_if_no_action":"无需 action"}`,
			},
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{ID: agentID, TenantID: tenantID, Name: "Planner"},
			Tools: []tool.Spec{
				{ID: toolID, TenantID: tenantID, Name: "search", Status: tool.StatusEnabled},
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, modelClient, configLoader)

	_, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text":                        "规划 action",
			runtimeDisableToolsPayloadKey: true,
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if len(modelClient.requests[0].Tools) != 0 {
		t.Fatalf("expected runtime tools disabled, got %#v", modelClient.requests[0].Tools)
	}
	if modelClient.requests[0].ToolChoice != "none" {
		t.Fatalf("expected tool_choice none, got %q", modelClient.requests[0].ToolChoice)
	}
}

func TestHandleEventPassesAgentModelConfigToModelGateway(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	modelClient := &fakeModelClient{
		response: model.ChatResponse{
			Message: model.Message{
				Role:    model.RoleAssistant,
				Content: "DeepSeek 回复",
			},
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{
				ID:       agentID,
				TenantID: tenantID,
				Name:     "Weather Agent",
				ModelConfig: agent.ModelConfig{
					ProviderID:  id.ID("provider_deepseek"),
					Model:       "deepseek-v4-flash",
					Temperature: 0.3,
				},
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, modelClient, configLoader)

	if _, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload:  map[string]any{"text": "你好"},
	}); err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if got := modelClient.requests[0].Model; got != "deepseek-v4-flash" {
		t.Fatalf("expected model deepseek-v4-flash, got %q", got)
	}
	if got := modelClient.requests[0].Options.Temperature; got != 0.3 {
		t.Fatalf("expected temperature 0.3, got %f", got)
	}
}

func TestHandleEventLoadsConversationHistoryForSameSession(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	sessionID := id.ID("session_1")
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "第一轮回复",
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "第二轮回复",
				},
			},
		},
	}
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		modelClient,
		nil,
		WithConversationStore(newFakeConversationStore()),
	)

	if _, err := service.HandleEvent(context.Background(), event.Event{
		TenantID:  tenantID,
		AgentID:   agentID,
		SessionID: sessionID,
		Type:      event.TypeUserMessageCommitted,
		Payload:   map[string]any{"text": "你好"},
	}); err != nil {
		t.Fatalf("first HandleEvent() error = %v", err)
	}
	if _, err := service.HandleEvent(context.Background(), event.Event{
		TenantID:  tenantID,
		AgentID:   agentID,
		SessionID: sessionID,
		Type:      event.TypeUserMessageCommitted,
		Payload:   map[string]any{"text": "继续"},
	}); err != nil {
		t.Fatalf("second HandleEvent() error = %v", err)
	}

	if len(modelClient.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(modelClient.requests))
	}
	messages := modelClient.requests[1].Messages
	if len(messages) != 4 {
		t.Fatalf("expected system + previous turn + current user, got %#v", messages)
	}
	if messages[1].Role != model.RoleUser || messages[1].Content != "你好" {
		t.Fatalf("expected first history user message, got %#v", messages[1])
	}
	if messages[2].Role != model.RoleAssistant || messages[2].Content != "第一轮回复" {
		t.Fatalf("expected first history assistant message, got %#v", messages[2])
	}
	if messages[3].Role != model.RoleUser || messages[3].Content != "继续" {
		t.Fatalf("expected current user message, got %#v", messages[3])
	}
}

func TestHandleEventRunsModelToolCallingWithAgentConfig(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	toolID := id.ID("tool_query_order")
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: model.ToolCallFunction{
								Name: "query_order",
								Arguments: map[string]any{
									"order_id": "o_123",
								},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "订单已查到",
				},
			},
		},
	}
	runtime := &fakeToolRuntime{
		result: tool.Result{
			ToolID:     toolID,
			Success:    true,
			Data:       map[string]any{"status": "paid"},
			FinishedAt: time.Now().UTC(),
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{
				ID:       agentID,
				TenantID: tenantID,
				Name:     "Order Agent",
			},
			Skills: []skill.Spec{
				{
					ID:       id.ID("skill_order"),
					TenantID: tenantID,
					Name:     "order",
					ToolIDs:  []id.ID{toolID},
				},
			},
			Tools: []tool.Spec{
				{
					ID:             toolID,
					TenantID:       tenantID,
					Name:           "query_order",
					Description:    "查询订单",
					Implementation: tool.ImplementationConnector,
					InputSchema: map[string]any{
						"type": "object",
					},
				},
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), runtime, modelClient, configLoader)

	resp, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text": "帮我查订单 o_123",
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if runtime.call.ToolID != toolID {
		t.Fatalf("expected tool id %q, got %q", toolID, runtime.call.ToolID)
	}
	if len(modelClient.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(modelClient.requests))
	}
	if !modelRequestHasTool(modelClient.requests[0], "query_order") {
		t.Fatalf("expected first model request to include query_order, got %#v", modelClient.requests[0].Tools)
	}
	if resp.Actions[0].Text != "订单已查到" {
		t.Fatalf("expected final reply, got %q", resp.Actions[0].Text)
	}
}

func TestHandleEventUsesAgentRuntimePolicyForMultiStepToolCalling(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_doc_upload")
	getPageToolID := id.ID("tool_get_page")
	uploadToolID := id.ID("tool_create_markdown_document")
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "call_get_page",
							Type: "function",
							Function: model.ToolCallFunction{
								Name:      "jc-confluence_get_page",
								Arguments: map[string]any{"page_id": "2973765212"},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "call_upload",
							Type: "function",
							Function: model.ToolCallFunction{
								Name: "create_markdown_document",
								Arguments: map[string]any{
									"title":   "Confluence 总结",
									"content": "## Summary\n...",
								},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "文档已上传",
				},
			},
		},
	}
	runtime := &fakeToolRuntime{
		result: tool.Result{
			Success:    true,
			Data:       map[string]any{"ok": true},
			FinishedAt: time.Now().UTC(),
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{
				ID:           agentID,
				TenantID:     tenantID,
				Name:         "文档上传测试",
				SystemPrompt: "先获取 Confluence 文档，再上传 Markdown 文档。",
				RuntimePolicy: agent.RuntimePolicy{
					MaxToolCalls: 2,
				},
			},
			Tools: []tool.Spec{
				{
					ID:          getPageToolID,
					TenantID:    tenantID,
					Name:        "jc-confluence_get_page",
					Description: "获取 Confluence 页面",
				},
				{
					ID:          uploadToolID,
					TenantID:    tenantID,
					Name:        "create_markdown_document",
					Description: "上传 Markdown 到飞书文档",
				},
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), runtime, modelClient, configLoader)

	resp, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload:  map[string]any{"text": "总结 Confluence 文档并上传到飞书"},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if resp.Actions[0].Text != "文档已上传" {
		t.Fatalf("expected final reply, got %q", resp.Actions[0].Text)
	}
	if len(runtime.calls) != 2 {
		t.Fatalf("expected two tool calls, got %d", len(runtime.calls))
	}
	if runtime.calls[0].ToolID != getPageToolID || runtime.calls[1].ToolID != uploadToolID {
		t.Fatalf("unexpected tool sequence %#v", runtime.calls)
	}
	if len(modelClient.requests) != 3 {
		t.Fatalf("expected two tool iterations plus final answer, got %d requests", len(modelClient.requests))
	}
	for i := 0; i < 2; i++ {
		if modelClient.requests[i].ToolChoice != "auto" || len(modelClient.requests[i].Tools) != 2 {
			t.Fatalf("expected request %d to keep tools enabled, got choice=%q tools=%d", i, modelClient.requests[i].ToolChoice, len(modelClient.requests[i].Tools))
		}
	}
	if modelClient.requests[2].ToolChoice != "none" || len(modelClient.requests[2].Tools) != 0 {
		t.Fatalf("expected final answer request to disable tools, got choice=%q tools=%d", modelClient.requests[2].ToolChoice, len(modelClient.requests[2].Tools))
	}
}

func TestHandleEventExecutesSelectedSkillWithOnlySkillTools(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	skillID := id.ID("skill_feishu")
	createToolID := id.ID("tool_feishu_create_document")
	convertToolID := id.ID("tool_feishu_convert_markdown")
	otherToolID := id.ID("tool_other")
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: model.ToolCallFunction{
								Name:      "feishu_create_document",
								Arguments: map[string]any{"title": "AI Report"},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "call_2",
							Type: "function",
							Function: model.ToolCallFunction{
								Name:      "feishu_convert_markdown",
								Arguments: map[string]any{"document_id": "doc_1", "content": "# Report"},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "飞书文档已上传",
				},
			},
		},
	}
	runtime := &fakeToolRuntime{
		result: tool.Result{
			Success:    true,
			Data:       map[string]any{"ok": true},
			FinishedAt: time.Now().UTC(),
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{
				ID:           agentID,
				TenantID:     tenantID,
				Name:         "Doc Agent",
				SystemPrompt: "agent prompt should not drive selected skill",
			},
			Skills: []skill.Spec{
				{
					ID:           skillID,
					TenantID:     tenantID,
					Name:         "feishu_doc_write",
					SystemPrompt: "创建文档后继续转换 Markdown。",
					ToolIDs:      []id.ID{createToolID, convertToolID},
					ExecutionPolicy: skill.ExecutionPolicy{
						MaxToolCalls: 4,
					},
				},
				{
					ID:       id.ID("skill_other"),
					TenantID: tenantID,
					Name:     "other",
					ToolIDs:  []id.ID{otherToolID},
				},
			},
			Tools: []tool.Spec{
				{ID: createToolID, TenantID: tenantID, Name: "feishu_create_document", Status: tool.StatusEnabled},
				{ID: convertToolID, TenantID: tenantID, Name: "feishu_convert_markdown", Status: tool.StatusEnabled},
				{ID: otherToolID, TenantID: tenantID, Name: "other_tool", Status: tool.StatusEnabled},
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), runtime, modelClient, configLoader)

	resp, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text":     "上传 Markdown 到飞书",
			"skill_id": skillID.String(),
			"skill_input": map[string]any{
				"title":        "AI Report",
				"content":      "# Report",
				"folder_token": "fld_selected",
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if resp.Actions[0].Text != "飞书文档已上传" {
		t.Fatalf("expected skill final reply, got %q", resp.Actions[0].Text)
	}
	if len(runtime.calls) != 2 {
		t.Fatalf("expected two skill tool calls, got %d", len(runtime.calls))
	}
	if runtime.calls[0].ToolID != createToolID || runtime.calls[1].ToolID != convertToolID {
		t.Fatalf("unexpected skill tool sequence %#v", runtime.calls)
	}
	if got := runtime.calls[0].Args["folder_token"]; got != "fld_selected" {
		t.Fatalf("expected folder_token default to be injected into create document call, got %#v", got)
	}
	if len(modelClient.requests) != 3 {
		t.Fatalf("expected three model requests, got %d", len(modelClient.requests))
	}
	if got := len(modelClient.requests[0].Tools); got != 2 {
		t.Fatalf("expected only selected skill tools, got %d", got)
	}
	for _, toolDef := range modelClient.requests[0].Tools {
		if toolDef.Function.Name == "other_tool" {
			t.Fatal("selected skill execution leaked tools from another skill")
		}
	}
	if !strings.Contains(modelClient.requests[0].Messages[0].Content, "Runtime Skill Action Contract") {
		t.Fatalf("expected skill execution prompt, got %q", modelClient.requests[0].Messages[0].Content)
	}
	if strings.Contains(modelClient.requests[0].Messages[0].Content, "agent prompt should not drive selected skill") {
		t.Fatalf("skill execution should not use parent agent prompt: %q", modelClient.requests[0].Messages[0].Content)
	}
}

func TestHandleEventCanInvokeSkillAsVirtualTool(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	skillID := id.ID("skill_feishu")
	createToolID := id.ID("tool_feishu_create_document")
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "skill_call_1",
							Type: "function",
							Function: model.ToolCallFunction{
								Name: "feishu_doc_write",
								Arguments: map[string]any{
									"task":  "上传 Markdown 到飞书",
									"input": map[string]any{"title": "AI Report", "content": "# Report", "folder_token": "fld_virtual"},
								},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "tool_call_1",
							Type: "function",
							Function: model.ToolCallFunction{
								Name:      "feishu_create_document",
								Arguments: map[string]any{"title": "AI Report"},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "skill 内部执行完成",
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "最终回复：飞书文档已上传",
				},
			},
		},
	}
	runtime := &fakeToolRuntime{
		result: tool.Result{
			ToolID:     createToolID,
			Success:    true,
			Data:       map[string]any{"document_id": "doc_1"},
			FinishedAt: time.Now().UTC(),
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{
				ID:       agentID,
				TenantID: tenantID,
				Name:     "Doc Agent",
			},
			Skills: []skill.Spec{
				{
					ID:           skillID,
					TenantID:     tenantID,
					Name:         "feishu_doc_write",
					Description:  "上传 Markdown 到飞书文档",
					SystemPrompt: "先创建文档，再写入内容。",
					ToolIDs:      []id.ID{createToolID},
					ExecutionPolicy: skill.ExecutionPolicy{
						MaxToolCalls: 3,
					},
				},
			},
			Tools: []tool.Spec{
				{ID: createToolID, TenantID: tenantID, Name: "feishu_create_document", Status: tool.StatusEnabled},
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), runtime, modelClient, configLoader)

	resp, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload:  map[string]any{"text": "帮我上传 Markdown 到飞书"},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if resp.Actions[0].Text != "最终回复：飞书文档已上传" {
		t.Fatalf("expected final reply, got %q", resp.Actions[0].Text)
	}
	if !modelRequestHasTool(modelClient.requests[0], "feishu_doc_write") {
		t.Fatalf("expected parent agent to see skill virtual tool, got %#v", modelClient.requests[0].Tools)
	}
	if !modelRequestHasTool(modelClient.requests[1], "feishu_create_document") {
		t.Fatalf("expected skill runtime to see skill tool, got %#v", modelClient.requests[1].Tools)
	}
	if modelRequestHasTool(modelClient.requests[1], "feishu_doc_write") {
		t.Fatalf("skill runtime must not recursively expose skill virtual tool, got %#v", modelClient.requests[1].Tools)
	}
	if len(runtime.calls) != 1 || runtime.calls[0].ToolID != createToolID {
		t.Fatalf("expected skill internal tool execution, got %#v", runtime.calls)
	}
	if got := runtime.calls[0].Args["folder_token"]; got != "fld_virtual" {
		t.Fatalf("expected virtual skill folder_token to be injected into tool call, got %#v", got)
	}
}

func TestFeishuSkillDefaultInputReadsFolderTokenEnv(t *testing.T) {
	t.Setenv("FEISHU_DOC_FOLDER_TOKEN", "fld_from_env")

	input := mergeSkillInputDefaults(skillDefaultInput(skill.Spec{
		ID:   id.ID("skill_feishu_doc_write"),
		Name: "feishu_doc_write",
	}), nil)

	if got := input["folder_token"]; got != "fld_from_env" {
		t.Fatalf("expected folder_token from env, got %#v", got)
	}
}

func TestFeishuWorkflowToolDefaultArgsReadsFolderTokenEnv(t *testing.T) {
	t.Setenv("FEISHU_DOC_FOLDER_TOKEN", "fld_workflow")

	args := defaultToolArgsForSpec(tool.Spec{
		ID:   id.ID("tool_feishu_upload_markdown"),
		Name: "feishu_upload_markdown",
		Binding: tool.Binding{
			WorkflowID: id.ID("wf_feishu_upload_markdown"),
		},
	})

	if got := args["folder_token"]; got != "fld_workflow" {
		t.Fatalf("expected workflow tool folder_token from env, got %#v", got)
	}
}

func TestMergeToolCallArgsKeepsNonEmptyDefaultWhenModelPassesEmptyValue(t *testing.T) {
	args := mergeToolCallArgs(
		map[string]any{"folder_token": "fld_default"},
		map[string]any{"folder_token": "", "title": "AI Report"},
	)

	if got := args["folder_token"]; got != "fld_default" {
		t.Fatalf("expected empty model value not to override default folder_token, got %#v", got)
	}
	if got := args["title"]; got != "AI Report" {
		t.Fatalf("expected title to be preserved, got %#v", got)
	}
}

func TestHandleEventInjectsFeishuFolderTokenForDirectTool(t *testing.T) {
	t.Setenv("FEISHU_DOC_FOLDER_TOKEN", "fld_direct")

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	createToolID := id.ID("tool_feishu_create_document")
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "tool_call_1",
							Type: "function",
							Function: model.ToolCallFunction{
								Name:      "feishu_create_document",
								Arguments: map[string]any{"title": "AI Report"},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "文档已创建",
				},
			},
		},
	}
	runtime := &fakeToolRuntime{
		result: tool.Result{
			ToolID:     createToolID,
			Success:    true,
			Data:       map[string]any{"document_id": "doc_1"},
			FinishedAt: time.Now().UTC(),
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{ID: agentID, TenantID: tenantID, Name: "Doc Agent"},
			Tools: []tool.Spec{
				{ID: createToolID, TenantID: tenantID, Name: "feishu_create_document", Status: tool.StatusEnabled},
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), runtime, modelClient, configLoader)

	if _, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload:  map[string]any{"text": "创建飞书文档"},
	}); err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if got := runtime.calls[0].Args["folder_token"]; got != "fld_direct" {
		t.Fatalf("expected direct tool folder_token from env, got %#v", got)
	}
}

func TestHandleEventRecordsFullChainTrace(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	toolID := id.ID("tool_weather")
	traceID := "trace_full_chain"
	traceStore := infrastructure.NewMemoryTraceStore()
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Provider:    "deepseek",
				ProviderURL: "https://api.deepseek.com",
				Model:       "deepseek-v4-flash",
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: model.ToolCallFunction{
								Name:      "query_weather",
								Arguments: map[string]any{"city": "深圳"},
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Provider:    "deepseek",
				ProviderURL: "https://api.deepseek.com",
				Model:       "deepseek-v4-flash",
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "深圳今天多云。",
				},
				FinishReason: "stop",
				Usage: model.Usage{
					InputTokens:  10,
					OutputTokens: 5,
					TotalTokens:  15,
				},
			},
		},
	}
	runtime := &fakeToolRuntime{
		result: tool.Result{
			ToolID:     toolID,
			Success:    true,
			Data:       map[string]any{"condition": "cloudy"},
			FinishedAt: time.Now().UTC(),
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{
				ID:       agentID,
				TenantID: tenantID,
				Name:     "Weather Agent",
				ModelConfig: agent.ModelConfig{
					ProviderID: id.ID("provider_deepseek"),
					Model:      "deepseek-v4-flash",
				},
			},
			Skills: []skill.Spec{
				{
					ID:       id.ID("skill_weather"),
					TenantID: tenantID,
					Name:     "weather",
					ToolIDs:  []id.ID{toolID},
				},
			},
			Tools: []tool.Spec{
				{
					ID:             toolID,
					TenantID:       tenantID,
					Name:           "query_weather",
					Implementation: tool.ImplementationConnector,
					Binding: tool.Binding{
						ConnectorOperationID: id.ID("connop_query_weather"),
					},
				},
			},
		},
	}
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		runtime,
		modelClient,
		configLoader,
		WithTraceStore(traceStore),
	)

	_, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		TraceID:  traceID,
		Type:     event.TypeUserMessageCommitted,
		Payload:  map[string]any{"text": "查深圳天气"},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}

	record, err := service.GetTrace(context.Background(), tenantID, traceID)
	if err != nil {
		t.Fatalf("GetTrace() error = %v", err)
	}
	if record.Status != domain.TraceStatusSucceeded {
		t.Fatalf("expected trace succeeded, got %q", record.Status)
	}
	assertTraceHasStep(t, record, domain.TraceStepAgent)
	assertTraceHasStep(t, record, domain.TraceStepSkill)
	assertTraceHasStep(t, record, domain.TraceStepTool)
	assertTraceHasStep(t, record, domain.TraceStepConnector)

	modelStep := traceStepByType(record, domain.TraceStepModel)
	if modelStep == nil {
		t.Fatal("expected model trace step")
	}
	if modelStep.Metadata["provider"] != "deepseek" {
		t.Fatalf("expected deepseek provider, got %#v", modelStep.Metadata["provider"])
	}
	if modelStep.Metadata["response_model"] != "deepseek-v4-flash" {
		t.Fatalf("expected deepseek-v4-flash response model, got %#v", modelStep.Metadata["response_model"])
	}
}

func TestHandleEventExpandsWorkflowToolTrace(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_doc")
	toolID := id.ID("tool_create_markdown_document")
	workflowID := id.ID("wf_create_markdown")
	traceID := "trace_workflow_tool"
	now := time.Now().UTC()
	nodeFinishedAt := now.Add(15 * time.Millisecond)
	runFinishedAt := now.Add(30 * time.Millisecond)
	traceStore := infrastructure.NewMemoryTraceStore()
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: model.ToolCallFunction{
								Name: "create_markdown_document",
								Arguments: map[string]any{
									"document_title":   "Doc",
									"markdown_content": "# Summary",
								},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "文档已上传",
				},
			},
		},
	}
	runtime := &fakeToolRuntime{
		result: tool.Result{
			ToolID:     toolID,
			Success:    true,
			Data:       map[string]any{"document_id": "doc_1"},
			StartedAt:  now,
			FinishedAt: runFinishedAt,
			WorkflowRun: &workflow.Run{
				ID:         id.ID("wfrun_1"),
				TenantID:   tenantID,
				WorkflowID: workflowID,
				Status:     workflow.RunStatusSucceeded,
				Input:      map[string]any{"document_title": "Doc"},
				Output:     map[string]any{"document_id": "doc_1"},
				StartedAt:  now,
				FinishedAt: &runFinishedAt,
			},
			WorkflowNodeRuns: []workflow.NodeRun{
				{
					ID:         id.ID("wfnoderun_1"),
					TenantID:   tenantID,
					RunID:      id.ID("wfrun_1"),
					WorkflowID: workflowID,
					NodeID:     id.ID("connop_create_doc"),
					NodeType:   workflow.NodeTypeConnectorOperation,
					NodeName:   "Create Document",
					Status:     workflow.NodeRunStatusSucceeded,
					Input:      map[string]any{"title": "Doc"},
					Output:     map[string]any{"document_id": "doc_1"},
					StartedAt:  now,
					FinishedAt: &nodeFinishedAt,
				},
			},
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{ID: agentID, TenantID: tenantID, Name: "Doc Agent"},
			Tools: []tool.Spec{
				{
					ID:             toolID,
					TenantID:       tenantID,
					Name:           "create_markdown_document",
					Implementation: tool.ImplementationWorkflow,
					Status:         tool.StatusEnabled,
					Binding: tool.Binding{
						WorkflowID: workflowID,
					},
				},
			},
		},
	}
	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		runtime,
		modelClient,
		configLoader,
		WithTraceStore(traceStore),
	)

	if _, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		TraceID:  traceID,
		Type:     event.TypeUserMessageCommitted,
		Payload:  map[string]any{"text": "上传文档"},
	}); err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}

	record, err := service.GetTrace(context.Background(), tenantID, traceID)
	if err != nil {
		t.Fatalf("GetTrace() error = %v", err)
	}
	toolStep := traceStepByType(record, domain.TraceStepTool)
	if toolStep == nil {
		t.Fatal("expected tool trace step")
	}
	workflowStep := traceStepByType(record, domain.TraceStepWorkflow)
	if workflowStep == nil {
		t.Fatal("expected workflow trace step")
	}
	if workflowStep.ParentID != toolStep.ID {
		t.Fatalf("expected workflow step parent %q, got %q", toolStep.ID, workflowStep.ParentID)
	}
	connectorStep := traceStepByType(record, domain.TraceStepConnector)
	if connectorStep == nil {
		t.Fatal("expected workflow connector node trace step")
	}
	if connectorStep.ParentID != workflowStep.ID {
		t.Fatalf("expected connector node parent %q, got %q", workflowStep.ID, connectorStep.ParentID)
	}
	if connectorStep.Metadata["node_id"] != "connop_create_doc" {
		t.Fatalf("expected workflow node metadata, got %#v", connectorStep.Metadata)
	}
}

func TestHandleEventExecutesModelSelectedHighRiskToolWithoutConfirmation(t *testing.T) {
	t.Parallel()

	tenantID := tenant.ID("tenant_1")
	agentID := id.ID("agent_1")
	toolID := id.ID("tool_refund_order")
	modelClient := &fakeModelClient{
		responses: []model.ChatResponse{
			{
				Message: model.Message{
					Role: model.RoleAssistant,
					ToolCalls: []model.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: model.ToolCallFunction{
								Name:      "refund_order",
								Arguments: map[string]any{"order_id": "o_123"},
							},
						},
					},
				},
			},
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "退款已执行",
				},
			},
		},
	}
	runtime := &fakeToolRuntime{
		result: tool.Result{
			ToolID:     toolID,
			Success:    true,
			FinishedAt: time.Now().UTC(),
		},
	}
	configLoader := &fakeAgentConfigLoader{
		config: domain.AgentConfig{
			Agent: agent.Profile{
				ID:       agentID,
				TenantID: tenantID,
				Name:     "Order Agent",
			},
			Tools: []tool.Spec{
				{
					ID:             toolID,
					TenantID:       tenantID,
					Name:           "refund_order",
					Description:    "退款",
					Implementation: tool.ImplementationConnector,
					RiskLevel:      tool.RiskHigh,
				},
			},
		},
	}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), runtime, modelClient, configLoader)

	resp, err := service.HandleEvent(context.Background(), event.Event{
		TenantID: tenantID,
		AgentID:  agentID,
		Type:     event.TypeUserMessageCommitted,
		Payload: map[string]any{
			"text": "帮我给订单 o_123 退款",
		},
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if resp.Actions[0].Text != "退款已执行" {
		t.Fatalf("expected final reply, got %q", resp.Actions[0].Text)
	}
	if len(runtime.calls) != 1 || runtime.calls[0].ToolID != toolID {
		t.Fatalf("expected high-risk tool to execute without confirmation, got %#v", runtime.calls)
	}
}

type fakeToolRuntime struct {
	call   tool.Call
	calls  []tool.Call
	result tool.Result
	err    error
}

func (r *fakeToolRuntime) ExecuteTool(ctx context.Context, call tool.Call) (tool.Result, error) {
	r.call = call
	r.calls = append(r.calls, call)
	if r.err != nil {
		return r.result, r.err
	}
	r.result.CallID = call.ID
	if r.result.ToolID.Empty() {
		r.result.ToolID = call.ToolID
	}
	return r.result, nil
}

type fakeModelClient struct {
	requests  []model.ChatRequest
	responses []model.ChatResponse
	response  model.ChatResponse
}

func (c *fakeModelClient) Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error) {
	c.requests = append(c.requests, req)
	if len(c.requests) == 1 {
		// Keep the original single-request test readable.
		c.response.RequestID = req.ID
	}
	if len(c.responses) > 0 {
		resp := c.responses[0]
		c.responses = c.responses[1:]
		return resp, nil
	}
	return c.response, nil
}

func assertTraceHasStep(t *testing.T, record domain.TraceRecord, stepType domain.TraceStepType) {
	t.Helper()
	if traceStepByType(record, stepType) == nil {
		t.Fatalf("expected trace step type %q in %#v", stepType, record.Steps)
	}
}

func traceStepByType(record domain.TraceRecord, stepType domain.TraceStepType) *domain.TraceStep {
	for index := range record.Steps {
		if record.Steps[index].Type == stepType {
			return &record.Steps[index]
		}
	}
	return nil
}

func modelRequestHasTool(req model.ChatRequest, name string) bool {
	for _, toolDef := range req.Tools {
		if toolDef.Function.Name == name {
			return true
		}
	}
	return false
}

type fakeAgentConfigLoader struct {
	config domain.AgentConfig
}

func (l *fakeAgentConfigLoader) LoadAgentConfig(ctx context.Context, tenantID tenant.ID, agentID id.ID) (domain.AgentConfig, error) {
	return l.config, nil
}

type fakeConversationStore struct {
	messages map[string][]model.Message
}

func newFakeConversationStore() *fakeConversationStore {
	return &fakeConversationStore{messages: map[string][]model.Message{}}
}

func (s *fakeConversationStore) LoadMessages(ctx context.Context, ref domain.ConversationRef, limit int) ([]model.Message, error) {
	messages := s.messages[ref.Key()]
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	result := make([]model.Message, len(messages))
	copy(result, messages)
	return result, nil
}

func (s *fakeConversationStore) AppendMessages(ctx context.Context, ref domain.ConversationRef, messages []model.Message) error {
	for _, message := range messages {
		if message.Role == model.RoleSystem {
			continue
		}
		s.messages[ref.Key()] = append(s.messages[ref.Key()], message)
	}
	return nil
}
