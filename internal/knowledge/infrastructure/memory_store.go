package infrastructure

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"flow-anything/internal/platform/contracts/knowledge"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

type MemoryStore struct {
	mu             sync.RWMutex
	knowledgeBases map[string]knowledge.Base
	documents      map[string]knowledge.Document
	chunks         map[string][]knowledge.Chunk
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		knowledgeBases: make(map[string]knowledge.Base),
		documents:      make(map[string]knowledge.Document),
		chunks:         make(map[string][]knowledge.Chunk),
	}
}

func (s *MemoryStore) SaveKnowledgeBase(ctx context.Context, base knowledge.Base) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := knowledgeBucketKey(base.TenantID.String(), base.ID.String())
	current := s.knowledgeBases[key]
	if base.UpdatedAt.IsZero() {
		base.UpdatedAt = time.Now().UTC()
	}
	base.DocumentCount = current.DocumentCount
	base.ChunkCount = current.ChunkCount
	s.knowledgeBases[key] = base
	return nil
}

func (s *MemoryStore) GetKnowledgeBase(ctx context.Context, tenantID string, kbID string) (knowledge.Base, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	base, ok := s.knowledgeBases[knowledgeBucketKey(tenantID, kbID)]
	if !ok {
		return knowledge.Base{}, apperrors.New(apperrors.CodeNotFound, "knowledge base not found")
	}
	return base, nil
}

func (s *MemoryStore) ListKnowledgeBases(ctx context.Context, tenantID string) ([]knowledge.Base, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := tenantID + "/"
	result := make([]knowledge.Base, 0)
	for key, base := range s.knowledgeBases {
		if strings.HasPrefix(key, prefix) {
			result = append(result, base)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result, nil
}

func (s *MemoryStore) ListDocuments(ctx context.Context, tenantID string, kbID string) ([]knowledge.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := knowledgeBucketKey(tenantID, kbID) + "/"
	result := make([]knowledge.Document, 0)
	for key, document := range s.documents {
		if strings.HasPrefix(key, prefix) {
			result = append(result, document)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Title < result[j].Title
	})
	return result, nil
}

func (s *MemoryStore) IndexDocument(ctx context.Context, document knowledge.Document, chunks []knowledge.Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	docKey := documentKey(document.TenantID.String(), document.KBID.String(), document.ID.String())
	s.documents[docKey] = document

	bucketKey := knowledgeBucketKey(document.TenantID.String(), document.KBID.String())
	existing := s.chunks[bucketKey]
	filtered := make([]knowledge.Chunk, 0, len(existing)+len(chunks))
	for _, chunk := range existing {
		if chunk.DocID != document.ID {
			filtered = append(filtered, chunk)
		}
	}
	filtered = append(filtered, chunks...)
	s.chunks[bucketKey] = filtered
	s.upsertKnowledgeBaseStatsLocked(document, filtered)

	return nil
}

func (s *MemoryStore) Search(ctx context.Context, query knowledge.Query) (knowledge.Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := s.candidateChunks(query)
	scored := make([]knowledge.Chunk, 0, len(candidates))
	for _, chunk := range candidates {
		if !metadataMatches(chunk.Metadata, query.Filters) {
			continue
		}
		score := lexicalScore(query.Text, chunk.Text, chunk.Metadata)
		if score <= 0 {
			continue
		}
		chunk.Score = score
		scored = append(scored, chunk)
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if query.TopK > 0 && len(scored) > query.TopK {
		scored = scored[:query.TopK]
	}

	return knowledge.Result{
		QueryID: query.ID,
		Chunks:  scored,
	}, nil
}

func (s *MemoryStore) candidateChunks(query knowledge.Query) []knowledge.Chunk {
	if len(query.KBIDs) == 0 {
		prefix := query.TenantID.String() + "/"
		result := make([]knowledge.Chunk, 0)
		for key, chunks := range s.chunks {
			if strings.HasPrefix(key, prefix) {
				if base, ok := s.knowledgeBases[key]; ok && base.Status == knowledge.BaseStatusDisabled {
					continue
				}
				result = append(result, chunks...)
			}
		}
		return result
	}

	result := make([]knowledge.Chunk, 0)
	for _, kbID := range query.KBIDs {
		key := knowledgeBucketKey(query.TenantID.String(), kbID.String())
		if base, ok := s.knowledgeBases[key]; ok && base.Status == knowledge.BaseStatusDisabled {
			continue
		}
		result = append(result, s.chunks[key]...)
	}
	return result
}

func (s *MemoryStore) upsertKnowledgeBaseStatsLocked(document knowledge.Document, bucketChunks []knowledge.Chunk) {
	key := knowledgeBucketKey(document.TenantID.String(), document.KBID.String())
	base := s.knowledgeBases[key]
	if base.ID.Empty() {
		base.ID = document.KBID
		base.TenantID = document.TenantID
		base.Name = document.KBID.String()
		base.Status = knowledge.BaseStatusEnabled
		base.Version = "v1"
	}
	if base.Status == "" {
		base.Status = knowledge.BaseStatusEnabled
	}
	docPrefix := key + "/"
	documentIDs := map[string]struct{}{}
	for docKey := range s.documents {
		if strings.HasPrefix(docKey, docPrefix) {
			documentIDs[strings.TrimPrefix(docKey, docPrefix)] = struct{}{}
		}
	}
	base.DocumentCount = len(documentIDs)
	base.ChunkCount = len(bucketChunks)
	base.UpdatedAt = time.Now().UTC()
	s.knowledgeBases[key] = base
}

func lexicalScore(query string, text string, metadata map[string]any) float64 {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	normalizedText := strings.ToLower(text)
	score := 0.0

	if strings.Contains(normalizedText, normalizedQuery) {
		score += 10
	}
	for _, term := range queryTerms(normalizedQuery) {
		if term == "" {
			continue
		}
		count := strings.Count(normalizedText, term)
		score += float64(count)
		if title, _ := metadata["title"].(string); strings.Contains(strings.ToLower(title), term) {
			score += 2
		}
	}

	return score
}

func queryTerms(query string) []string {
	rawTerms := strings.FieldsFunc(query, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	terms := make([]string, 0, len(rawTerms)*2)
	for _, term := range rawTerms {
		if term == "" {
			continue
		}
		terms = append(terms, term)
		runes := []rune(term)
		if len(runes) > 4 {
			for i := 0; i < len(runes)-1; i++ {
				terms = append(terms, string(runes[i:i+2]))
			}
		}
	}
	if len(terms) > 0 {
		return terms
	}

	runes := []rune(query)
	if len(runes) <= 2 {
		return []string{query}
	}
	result := make([]string, 0, len(runes)-1)
	for i := 0; i < len(runes)-1; i++ {
		result = append(result, string(runes[i:i+2]))
	}
	return result
}

func metadataMatches(metadata map[string]any, filters map[string]any) bool {
	for key, expected := range filters {
		if expected == nil {
			continue
		}
		actual, ok := metadata[key]
		if !ok || strings.TrimSpace(toString(actual)) != strings.TrimSpace(toString(expected)) {
			return false
		}
	}
	return true
}

func documentKey(tenantID string, kbID string, docID string) string {
	return tenantID + "/" + kbID + "/" + docID
}

func knowledgeBucketKey(tenantID string, kbID string) string {
	return tenantID + "/" + kbID
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}
