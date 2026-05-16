package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"flow-anything/internal/platform/contracts/knowledge"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHTTPKnowledgeRetrieverSearch(t *testing.T) {
	t.Parallel()

	client := httpclient.NewWithHTTPClient("http://knowledge.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/knowledge/search" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}

			var req knowledge.Query
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if req.Text != "退款" {
				t.Fatalf("expected query text, got %q", req.Text)
			}

			var body strings.Builder
			_ = json.NewEncoder(&body).Encode(knowledge.Result{
				QueryID: req.ID,
				Chunks: []knowledge.Chunk{
					{
						ID:    id.ID("chunk_1"),
						DocID: id.ID("doc_1"),
						KBID:  id.ID("kb_help"),
						Text:  "退款规则",
						Score: 2,
					},
				},
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body.String())),
			}, nil
		}),
	})

	retriever := &HTTPKnowledgeRetriever{client: client}
	result, err := retriever.Search(context.Background(), knowledge.Query{
		ID:       id.ID("kbq_1"),
		TenantID: tenant.ID("tenant_1"),
		Text:     "退款",
		TopK:     3,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("expected one chunk, got %d", len(result.Chunks))
	}
}
