package infrastructure

import (
	"context"
	"time"

	"flow-anything/internal/platform/contracts/knowledge"
	"flow-anything/internal/platform/kernel/httpclient"
)

type HTTPKnowledgeRetriever struct {
	client *httpclient.Client
}

func NewHTTPKnowledgeRetriever(knowledgeServiceBaseURL string) *HTTPKnowledgeRetriever {
	return &HTTPKnowledgeRetriever{
		client: httpclient.New(knowledgeServiceBaseURL, 10*time.Second),
	}
}

func (r *HTTPKnowledgeRetriever) Search(ctx context.Context, query knowledge.Query) (knowledge.Result, error) {
	var result knowledge.Result
	if err := r.client.PostJSON(ctx, "/v1/knowledge/search", query, &result); err != nil {
		return knowledge.Result{}, err
	}

	return result, nil
}
