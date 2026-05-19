package modeladapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"flow-anything/core/agentcore"
)

func TestOpenAICompatibleClientChat(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["model"] != "deepseek-v4-flash" {
			t.Fatalf("unexpected model: %#v", req["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl_test",
			"model": "deepseek-v4-flash",
			"choices": []map[string]any{
				{
					"message": map[string]any{"role": "assistant", "content": "hello"},
				},
			},
			"usage": map[string]any{"total_tokens": 3},
		})
	}))
	defer server.Close()

	client := OpenAICompatibleClient{BaseURL: server.URL, APIKey: "secret"}
	resp, err := client.Chat(context.Background(), agentcore.ModelRequest{
		Model: agentcore.ModelConfig{
			Provider: "deepseek",
			Model:    "deepseek-v4-flash",
		},
		Messages: []agentcore.Message{{Role: "user", Content: "hi"}},
		TraceID:  "trace_model",
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if authHeader != "Bearer secret" {
		t.Fatalf("unexpected auth header: %q", authHeader)
	}
	if resp.Message.Content != "hello" {
		t.Fatalf("unexpected content: %q", resp.Message.Content)
	}
	if resp.Provider != "deepseek" {
		t.Fatalf("unexpected provider: %q", resp.Provider)
	}
}
