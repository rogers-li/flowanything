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
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHTTPModelClientChat(t *testing.T) {
	t.Parallel()

	client := httpclient.NewWithHTTPClient("http://model.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/chat/completions" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}

			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if len(req.Messages) == 0 {
				t.Fatal("expected messages")
			}

			var body strings.Builder
			_ = json.NewEncoder(&body).Encode(model.ChatResponse{
				ID:        id.ID("chatcmpl_1"),
				RequestID: req.ID,
				Model:     "mock-chat",
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "hello",
				},
				CreatedAt: time.Now().UTC(),
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body.String())),
			}, nil
		}),
	})

	modelClient := &HTTPModelClient{client: client}
	resp, err := modelClient.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Message.Content != "hello" {
		t.Fatalf("expected hello, got %q", resp.Message.Content)
	}
}
