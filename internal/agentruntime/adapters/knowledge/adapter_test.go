package knowledge

import (
	"context"
	"testing"

	knowledgecontract "flow-anything/internal/platform/contracts/knowledge"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestAdapterExecutesKnowledgeSearch(t *testing.T) {
	t.Parallel()

	kbID := id.ID("kb_help")
	toolID := id.ID("tool_search_knowledge")
	retriever := &fakeRetriever{
		result: knowledgecontract.Result{
			Chunks: []knowledgecontract.Chunk{
				{
					ID:    id.ID("chunk_1"),
					DocID: id.ID("doc_1"),
					KBID:  kbID,
					Text:  "退款一般会在三个工作日内到账。",
					Score: 3,
				},
			},
		},
	}
	adapter := New(retriever)

	result, err := adapter.Execute(context.Background(), tool.Spec{
		ID:             toolID,
		Implementation: tool.ImplementationKnowledge,
		Binding: tool.Binding{
			KnowledgeBaseIDs: []id.ID{kbID},
		},
	}, tool.Call{
		ID:       id.ID("call_1"),
		TenantID: tenant.ID("tenant_1"),
		ToolID:   toolID,
		Args: map[string]any{
			"query": "退款多久到账",
			"top_k": float64(2),
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected successful result")
	}
	if retriever.query.Text != "退款多久到账" {
		t.Fatalf("expected query text, got %q", retriever.query.Text)
	}
	if len(retriever.query.KBIDs) != 1 || retriever.query.KBIDs[0] != kbID {
		t.Fatalf("unexpected kb ids %#v", retriever.query.KBIDs)
	}
	if result.Data["chunks"] == nil {
		t.Fatal("expected chunks in tool result data")
	}
}

type fakeRetriever struct {
	query  knowledgecontract.Query
	result knowledgecontract.Result
}

func (r *fakeRetriever) Search(ctx context.Context, query knowledgecontract.Query) (knowledgecontract.Result, error) {
	r.query = query
	r.result.QueryID = query.ID
	return r.result, nil
}
