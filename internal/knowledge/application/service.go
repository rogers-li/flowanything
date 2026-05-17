package application

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"flow-anything/internal/knowledge/domain"
	"flow-anything/internal/knowledge/ports"
	"flow-anything/internal/platform/contracts/knowledge"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type Service struct {
	logger    *slog.Logger
	indexer   ports.DocumentIndexer
	retriever ports.Retriever
	bases     ports.KnowledgeBaseRepository
}

func New(logger *slog.Logger, indexer ports.DocumentIndexer, retriever ports.Retriever, baseRepos ...ports.KnowledgeBaseRepository) *Service {
	var bases ports.KnowledgeBaseRepository
	if len(baseRepos) > 0 {
		bases = baseRepos[0]
	} else if repo, ok := indexer.(ports.KnowledgeBaseRepository); ok {
		bases = repo
	}
	return &Service{
		logger:    logger,
		indexer:   indexer,
		retriever: retriever,
		bases:     bases,
	}
}

func (s *Service) CreateKnowledgeBase(ctx context.Context, base knowledge.Base) (knowledge.Base, error) {
	if base.ID.Empty() {
		base.ID = id.New("kb")
	}
	if base.Version == "" {
		base.Version = "v1"
	}
	if base.Status == "" {
		base.Status = knowledge.BaseStatusDraft
	}
	base.Name = strings.TrimSpace(base.Name)
	base.Description = strings.TrimSpace(base.Description)
	base.UpdatedAt = time.Now().UTC()
	if err := domain.ValidateKnowledgeBase(base); err != nil {
		return knowledge.Base{}, err
	}
	if s.bases == nil {
		return knowledge.Base{}, apperrors.New(apperrors.CodeUnavailable, "knowledge base repository is not configured")
	}
	if err := s.bases.SaveKnowledgeBase(ctx, base); err != nil {
		return knowledge.Base{}, err
	}
	return base, nil
}

func (s *Service) UpdateKnowledgeBase(ctx context.Context, base knowledge.Base) (knowledge.Base, error) {
	if base.TenantID.Empty() {
		return knowledge.Base{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if base.ID.Empty() {
		return knowledge.Base{}, apperrors.New(apperrors.CodeInvalidArgument, "kb_id is required")
	}
	if s.bases == nil {
		return knowledge.Base{}, apperrors.New(apperrors.CodeUnavailable, "knowledge base repository is not configured")
	}
	current, err := s.bases.GetKnowledgeBase(ctx, base.TenantID.String(), base.ID.String())
	if err != nil {
		return knowledge.Base{}, err
	}
	if base.Version == "" {
		base.Version = current.Version
	}
	if base.Status == "" {
		base.Status = current.Status
	}
	base.DocumentCount = current.DocumentCount
	base.ChunkCount = current.ChunkCount
	base.UpdatedAt = time.Now().UTC()
	if err := domain.ValidateKnowledgeBase(base); err != nil {
		return knowledge.Base{}, err
	}
	if err := s.bases.SaveKnowledgeBase(ctx, base); err != nil {
		return knowledge.Base{}, err
	}
	return base, nil
}

func (s *Service) SetKnowledgeBaseStatus(ctx context.Context, tenantID tenant.ID, kbID id.ID, status knowledge.BaseStatus) (knowledge.Base, error) {
	if tenantID.Empty() {
		return knowledge.Base{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if kbID.Empty() {
		return knowledge.Base{}, apperrors.New(apperrors.CodeInvalidArgument, "kb_id is required")
	}
	if status != knowledge.BaseStatusEnabled && status != knowledge.BaseStatusDisabled {
		return knowledge.Base{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported knowledge base status transition")
	}
	if s.bases == nil {
		return knowledge.Base{}, apperrors.New(apperrors.CodeUnavailable, "knowledge base repository is not configured")
	}
	base, err := s.bases.GetKnowledgeBase(ctx, tenantID.String(), kbID.String())
	if err != nil {
		return knowledge.Base{}, err
	}
	base.Status = status
	base.UpdatedAt = time.Now().UTC()
	if err := s.bases.SaveKnowledgeBase(ctx, base); err != nil {
		return knowledge.Base{}, err
	}
	return base, nil
}

func (s *Service) ListKnowledgeBases(ctx context.Context, tenantID tenant.ID) ([]knowledge.Base, error) {
	if tenantID.Empty() {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if s.bases == nil {
		return []knowledge.Base{}, nil
	}
	return s.bases.ListKnowledgeBases(ctx, tenantID.String())
}

func (s *Service) GetKnowledgeBase(ctx context.Context, tenantID tenant.ID, kbID id.ID) (knowledge.Base, error) {
	if tenantID.Empty() {
		return knowledge.Base{}, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if kbID.Empty() {
		return knowledge.Base{}, apperrors.New(apperrors.CodeInvalidArgument, "kb_id is required")
	}
	if s.bases == nil {
		return knowledge.Base{}, apperrors.New(apperrors.CodeUnavailable, "knowledge base repository is not configured")
	}
	return s.bases.GetKnowledgeBase(ctx, tenantID.String(), kbID.String())
}

func (s *Service) ListDocuments(ctx context.Context, tenantID tenant.ID, kbID id.ID) ([]knowledge.Document, error) {
	if tenantID.Empty() {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if kbID.Empty() {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "kb_id is required")
	}
	if s.bases == nil {
		return []knowledge.Document{}, nil
	}
	return s.bases.ListDocuments(ctx, tenantID.String(), kbID.String())
}

func (s *Service) IndexDocument(ctx context.Context, document knowledge.Document) (knowledge.IndexResult, error) {
	if document.ID.Empty() {
		document.ID = id.New("doc")
	}
	if document.Version == "" {
		document.Version = "v1"
	}
	if err := domain.ValidateDocument(document); err != nil {
		return knowledge.IndexResult{}, err
	}

	chunks := domain.ChunkDocument(document)
	if s.indexer == nil {
		return knowledge.IndexResult{}, apperrors.New(apperrors.CodeUnavailable, "knowledge indexer is not configured")
	}
	if err := s.indexer.IndexDocument(ctx, document, chunks); err != nil {
		return knowledge.IndexResult{}, err
	}

	s.logger.Info("knowledge document indexed",
		"document_id", document.ID.String(),
		"kb_id", document.KBID.String(),
		"chunk_count", len(chunks),
	)

	return knowledge.IndexResult{
		DocumentID: document.ID,
		KBID:       document.KBID,
		ChunkCount: len(chunks),
		Chunks:     chunks,
	}, nil
}

func (s *Service) Search(ctx context.Context, query knowledge.Query) (knowledge.Result, error) {
	if query.ID.Empty() {
		query.ID = id.New("kbq")
	}
	if query.TopK == 0 {
		query.TopK = 5
	}
	if err := domain.ValidateQuery(query); err != nil {
		return knowledge.Result{}, err
	}
	if s.retriever == nil {
		return knowledge.Result{}, apperrors.New(apperrors.CodeUnavailable, "knowledge retriever is not configured")
	}

	s.logger.Info("knowledge search accepted",
		"query_id", query.ID.String(),
		"top_k", query.TopK,
	)

	return s.retriever.Search(ctx, query)
}
