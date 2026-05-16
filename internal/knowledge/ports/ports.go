package ports

import (
	"context"

	"flow-anything/internal/platform/contracts/knowledge"
)

type Retriever interface {
	Search(ctx context.Context, query knowledge.Query) (knowledge.Result, error)
}

type DocumentIndexer interface {
	IndexDocument(ctx context.Context, document knowledge.Document, chunks []knowledge.Chunk) error
}

type KnowledgeBaseRepository interface {
	SaveKnowledgeBase(ctx context.Context, base knowledge.Base) error
	GetKnowledgeBase(ctx context.Context, tenantID string, kbID string) (knowledge.Base, error)
	ListKnowledgeBases(ctx context.Context, tenantID string) ([]knowledge.Base, error)
	ListDocuments(ctx context.Context, tenantID string, kbID string) ([]knowledge.Document, error)
}
