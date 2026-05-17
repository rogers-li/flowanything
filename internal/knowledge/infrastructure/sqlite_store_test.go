package infrastructure

import (
	"context"
	"path/filepath"
	"testing"

	"flow-anything/internal/platform/contracts/knowledge"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestSQLiteStorePersistsKnowledgeBaseDocumentAndSearch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := OpenSQLiteStore(ctx, "file:"+filepath.Join(t.TempDir(), "knowledge.db")+"?cache=shared")
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	defer store.Close()

	base := knowledge.Base{
		ID:       id.ID("kb_help"),
		TenantID: tenant.ID("tenant_1"),
		Name:     "Help Center",
		Status:   knowledge.BaseStatusEnabled,
		Version:  "v1",
	}
	if err := store.SaveKnowledgeBase(ctx, base); err != nil {
		t.Fatalf("SaveKnowledgeBase() error = %v", err)
	}
	document := knowledge.Document{
		ID:       id.ID("doc_refund"),
		TenantID: tenant.ID("tenant_1"),
		KBID:     id.ID("kb_help"),
		Title:    "Refund policy",
		Text:     "Refund requests are handled within three business days.",
		Version:  "v1",
	}
	chunks := []knowledge.Chunk{{
		ID:    id.ID("doc_refund_chunk_0"),
		DocID: document.ID,
		KBID:  document.KBID,
		Text:  document.Text,
		Metadata: map[string]any{
			"title": document.Title,
		},
	}}
	if err := store.IndexDocument(ctx, document, chunks); err != nil {
		t.Fatalf("IndexDocument() error = %v", err)
	}

	bases, err := store.ListKnowledgeBases(ctx, "tenant_1")
	if err != nil {
		t.Fatalf("ListKnowledgeBases() error = %v", err)
	}
	if len(bases) != 1 || bases[0].DocumentCount != 1 || bases[0].ChunkCount != 1 {
		t.Fatalf("unexpected base stats: %#v", bases)
	}

	result, err := store.Search(ctx, knowledge.Query{
		ID:       id.ID("query_1"),
		TenantID: tenant.ID("tenant_1"),
		KBIDs:    []id.ID{id.ID("kb_help")},
		Text:     "refund business days",
		TopK:     3,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(result.Chunks) != 1 || result.Chunks[0].DocID != document.ID {
		t.Fatalf("unexpected search result: %#v", result)
	}
}
