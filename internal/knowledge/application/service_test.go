package application

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"flow-anything/internal/knowledge/infrastructure"
	"flow-anything/internal/platform/contracts/knowledge"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestIndexDocumentAndSearch(t *testing.T) {
	t.Parallel()

	store := infrastructure.NewMemoryStore()
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), store, store)

	indexResp, err := service.IndexDocument(context.Background(), knowledge.Document{
		ID:       id.ID("doc_refund"),
		TenantID: tenant.ID("tenant_1"),
		KBID:     id.ID("kb_help"),
		Title:    "退款规则",
		Text:     "用户可以在订单支付后七天内申请退款。退款一般会在三个工作日内到账。",
		Metadata: map[string]any{
			"locale": "zh-CN",
		},
	})
	if err != nil {
		t.Fatalf("IndexDocument() error = %v", err)
	}
	if indexResp.ChunkCount != 1 {
		t.Fatalf("expected one chunk, got %d", indexResp.ChunkCount)
	}

	searchResp, err := service.Search(context.Background(), knowledge.Query{
		TenantID: tenant.ID("tenant_1"),
		KBIDs:    []id.ID{id.ID("kb_help")},
		Text:     "退款多久到账",
		TopK:     3,
		Filters: map[string]any{
			"locale": "zh-CN",
		},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(searchResp.Chunks) != 1 {
		t.Fatalf("expected one chunk, got %d", len(searchResp.Chunks))
	}
	if searchResp.Chunks[0].DocID != id.ID("doc_refund") {
		t.Fatalf("expected refund document, got %q", searchResp.Chunks[0].DocID)
	}
}
