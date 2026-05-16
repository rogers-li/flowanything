package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestOpenAICompatibleProviderChat(t *testing.T) {
	t.Parallel()

	provider, err := NewOpenAICompatibleProviderWithClient(OpenAICompatibleConfig{
		BaseURL:      "https://model.test/v1",
		APIKey:       "test-key",
		DefaultModel: "test-model",
	}, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://model.test/v1/chat/completions" {
				t.Fatalf("unexpected url %q", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("unexpected authorization header %q", req.Header.Get("Authorization"))
			}

			var body openAIChatRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if body.Model != "test-model" {
				t.Fatalf("expected default model, got %q", body.Model)
			}
			if len(body.Messages) != 1 || body.Messages[0].Content != "hello" {
				t.Fatalf("unexpected messages %#v", body.Messages)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"id":"chatcmpl_1",
					"model":"test-model",
					"created":1710000000,
					"choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
					"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}
				}`)),
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProviderWithClient() error = %v", err)
	}

	resp, err := provider.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Message.Content != "hi" {
		t.Fatalf("expected hi, got %q", resp.Message.Content)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("expected 5 total tokens, got %d", resp.Usage.TotalTokens)
	}
	if !resp.CreatedAt.Equal(time.Unix(1710000000, 0).UTC()) {
		t.Fatalf("unexpected created_at %s", resp.CreatedAt)
	}
}

func TestOpenAICompatibleProviderChatMapsTools(t *testing.T) {
	t.Parallel()

	provider, err := NewOpenAICompatibleProviderWithClient(OpenAICompatibleConfig{
		BaseURL:      "https://model.test/v1",
		APIKey:       "test-key",
		DefaultModel: "test-model",
	}, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body openAIChatRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if body.ToolChoice != "auto" {
				t.Fatalf("expected tool_choice auto, got %q", body.ToolChoice)
			}
			if len(body.Tools) != 1 || body.Tools[0].Function.Name != "query_order" {
				t.Fatalf("unexpected tools %#v", body.Tools)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"id":"chatcmpl_1",
					"model":"test-model",
					"created":1710000000,
					"choices":[{
						"message":{
							"role":"assistant",
							"content":"",
							"tool_calls":[{
								"id":"call_1",
								"type":"function",
								"function":{
									"name":"query_order",
									"arguments":"{\"order_id\":\"o_123\"}"
								}
							}]
						},
						"finish_reason":"tool_calls"
					}]
				}`)),
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProviderWithClient() error = %v", err)
	}

	resp, err := provider.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "帮我查订单 o_123"},
		},
		Tools: []model.ToolDefinition{
			{
				Type: "function",
				Function: model.ToolFunction{
					Name: "query_order",
					Parameters: map[string]any{
						"type": "object",
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
		t.Fatalf("expected tool_calls finish reason, got %q", resp.FinishReason)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Function.Arguments["order_id"] != "o_123" {
		t.Fatalf("unexpected tool arguments %#v", resp.Message.ToolCalls[0].Function.Arguments)
	}
}

func TestOpenAICompatibleProviderMergesExtraBody(t *testing.T) {
	t.Parallel()

	provider, err := NewOpenAICompatibleProviderWithClient(OpenAICompatibleConfig{
		BaseURL:      "https://model.test",
		APIKey:       "test-key",
		DefaultModel: "test-model",
		ExtraBody: map[string]any{
			"thinking":         map[string]any{"type": "disabled"},
			"reasoning_effort": "high",
		},
	}, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			thinking, _ := body["thinking"].(map[string]any)
			if thinking["type"] != "disabled" {
				t.Fatalf("expected thinking disabled, got %#v", body["thinking"])
			}
			if body["reasoning_effort"] != "high" {
				t.Fatalf("expected reasoning effort high, got %#v", body["reasoning_effort"])
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"id":"chatcmpl_1",
					"model":"test-model",
					"created":1710000000,
					"choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]
				}`)),
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProviderWithClient() error = %v", err)
	}

	_, err = provider.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestOpenAICompatibleProviderRequiresConfig(t *testing.T) {
	t.Parallel()

	if _, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{}); err == nil {
		t.Fatal("expected config error")
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
