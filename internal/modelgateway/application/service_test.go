package application

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"flow-anything/internal/modelgateway/infrastructure"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestChatUsesProvider(t *testing.T) {
	t.Parallel()

	service := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		infrastructure.NewMockProvider(),
	)

	resp, err := service.Chat(context.Background(), model.ChatRequest{
		TenantID: tenant.ID("tenant_1"),
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "你好"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Message.Role != model.RoleAssistant {
		t.Fatalf("expected assistant message, got %q", resp.Message.Role)
	}
	if resp.Message.Content == "" {
		t.Fatal("expected non-empty content")
	}
}
