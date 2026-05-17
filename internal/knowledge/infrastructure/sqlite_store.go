package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"flow-anything/internal/platform/contracts/knowledge"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLiteStore(ctx context.Context, dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to open sqlite knowledge store", err)
	}
	db.SetMaxOpenConns(1)
	store := &SQLiteStore{db: db}
	if err := store.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS knowledge_bases (
			tenant_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			embedding_model TEXT NOT NULL DEFAULT '',
			document_count INTEGER NOT NULL DEFAULT 0,
			chunk_count INTEGER NOT NULL DEFAULT 0,
			metadata_json TEXT NOT NULL DEFAULT '{}',
			version TEXT NOT NULL DEFAULT 'v1',
			updated_at TEXT NOT NULL,
			PRIMARY KEY (tenant_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_documents (
			tenant_id TEXT NOT NULL,
			kb_id TEXT NOT NULL,
			id TEXT NOT NULL,
			title TEXT NOT NULL,
			text TEXT NOT NULL,
			metadata_json TEXT NOT NULL DEFAULT '{}',
			version TEXT NOT NULL DEFAULT 'v1',
			PRIMARY KEY (tenant_id, kb_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_chunks (
			tenant_id TEXT NOT NULL,
			kb_id TEXT NOT NULL,
			doc_id TEXT NOT NULL,
			id TEXT NOT NULL,
			text TEXT NOT NULL,
			metadata_json TEXT NOT NULL DEFAULT '{}',
			PRIMARY KEY (tenant_id, kb_id, doc_id, id)
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return apperrors.Wrap(apperrors.CodeInternal, "failed to migrate knowledge store", err)
		}
	}
	return nil
}

func (s *SQLiteStore) SaveKnowledgeBase(ctx context.Context, base knowledge.Base) error {
	metadata, err := encodeJSON(base.Metadata)
	if err != nil {
		return err
	}
	if base.UpdatedAt.IsZero() {
		base.UpdatedAt = time.Now().UTC()
	}
	counts, err := s.countStats(ctx, base.TenantID.String(), base.ID.String())
	if err != nil {
		return err
	}
	base.DocumentCount = counts.documents
	base.ChunkCount = counts.chunks
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO knowledge_bases (
			tenant_id, id, name, description, status, embedding_model,
			document_count, chunk_count, metadata_json, version, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			status = excluded.status,
			embedding_model = excluded.embedding_model,
			document_count = excluded.document_count,
			chunk_count = excluded.chunk_count,
			metadata_json = excluded.metadata_json,
			version = excluded.version,
			updated_at = excluded.updated_at
	`, base.TenantID.String(), base.ID.String(), base.Name, base.Description, string(base.Status), base.EmbeddingModel,
		base.DocumentCount, base.ChunkCount, metadata, base.Version, base.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save knowledge base", err)
	}
	return nil
}

func (s *SQLiteStore) GetKnowledgeBase(ctx context.Context, tenantID string, kbID string) (knowledge.Base, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT tenant_id, id, name, description, status, embedding_model,
			document_count, chunk_count, metadata_json, version, updated_at
		FROM knowledge_bases
		WHERE tenant_id = ? AND id = ?
	`, tenantID, kbID)
	return scanKnowledgeBase(row)
}

func (s *SQLiteStore) ListKnowledgeBases(ctx context.Context, tenantID string) ([]knowledge.Base, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, status, embedding_model,
			document_count, chunk_count, metadata_json, version, updated_at
		FROM knowledge_bases
		WHERE tenant_id = ?
		ORDER BY updated_at DESC
	`, tenantID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list knowledge bases", err)
	}
	defer rows.Close()

	result := make([]knowledge.Base, 0)
	for rows.Next() {
		base, err := scanKnowledgeBase(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, base)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) ListDocuments(ctx context.Context, tenantID string, kbID string) ([]knowledge.Document, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id, kb_id, id, title, text, metadata_json, version
		FROM knowledge_documents
		WHERE tenant_id = ? AND kb_id = ?
		ORDER BY title ASC
	`, tenantID, kbID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list knowledge documents", err)
	}
	defer rows.Close()

	result := make([]knowledge.Document, 0)
	for rows.Next() {
		document, err := scanKnowledgeDocument(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, document)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) IndexDocument(ctx context.Context, document knowledge.Document, chunks []knowledge.Chunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to start knowledge index transaction", err)
	}
	defer tx.Rollback()

	documentMetadata, err := encodeJSON(document.Metadata)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO knowledge_documents (tenant_id, kb_id, id, title, text, metadata_json, version)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, kb_id, id) DO UPDATE SET
			title = excluded.title,
			text = excluded.text,
			metadata_json = excluded.metadata_json,
			version = excluded.version
	`, document.TenantID.String(), document.KBID.String(), document.ID.String(), document.Title, document.Text, documentMetadata, document.Version); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save knowledge document", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM knowledge_chunks
		WHERE tenant_id = ? AND kb_id = ? AND doc_id = ?
	`, document.TenantID.String(), document.KBID.String(), document.ID.String()); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to replace knowledge chunks", err)
	}
	for _, chunk := range chunks {
		metadata, err := encodeJSON(chunk.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO knowledge_chunks (tenant_id, kb_id, doc_id, id, text, metadata_json)
			VALUES (?, ?, ?, ?, ?, ?)
		`, document.TenantID.String(), document.KBID.String(), chunk.DocID.String(), chunk.ID.String(), chunk.Text, metadata); err != nil {
			return apperrors.Wrap(apperrors.CodeInternal, "failed to save knowledge chunk", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to commit knowledge index", err)
	}
	return s.ensureBaseStats(ctx, document)
}

func (s *SQLiteStore) Search(ctx context.Context, query knowledge.Query) (knowledge.Result, error) {
	candidates, err := s.candidateChunks(ctx, query)
	if err != nil {
		return knowledge.Result{}, err
	}
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
	return knowledge.Result{QueryID: query.ID, Chunks: scored}, nil
}

func (s *SQLiteStore) candidateChunks(ctx context.Context, query knowledge.Query) ([]knowledge.Chunk, error) {
	args := []any{query.TenantID.String()}
	filter := "c.tenant_id = ? AND COALESCE(b.status, 'enabled') != 'disabled'"
	if len(query.KBIDs) > 0 {
		placeholders := make([]string, 0, len(query.KBIDs))
		for _, kbID := range query.KBIDs {
			placeholders = append(placeholders, "?")
			args = append(args, kbID.String())
		}
		filter += " AND c.kb_id IN (" + strings.Join(placeholders, ",") + ")"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.kb_id, c.doc_id, c.id, c.text, c.metadata_json
		FROM knowledge_chunks c
		LEFT JOIN knowledge_bases b ON b.tenant_id = c.tenant_id AND b.id = c.kb_id
		WHERE `+filter, args...)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to search knowledge chunks", err)
	}
	defer rows.Close()

	result := make([]knowledge.Chunk, 0)
	for rows.Next() {
		var kbID, docID, chunkID, text, metadataRaw string
		if err := rows.Scan(&kbID, &docID, &chunkID, &text, &metadataRaw); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to scan knowledge chunk", err)
		}
		metadata := map[string]any{}
		if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to decode knowledge chunk metadata", err)
		}
		result = append(result, knowledge.Chunk{
			ID:       id.ID(chunkID),
			DocID:    id.ID(docID),
			KBID:     id.ID(kbID),
			Text:     text,
			Metadata: metadata,
		})
	}
	return result, rows.Err()
}

type knowledgeStats struct {
	documents int
	chunks    int
}

func (s *SQLiteStore) countStats(ctx context.Context, tenantID string, kbID string) (knowledgeStats, error) {
	var stats knowledgeStats
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_documents WHERE tenant_id = ? AND kb_id = ?`, tenantID, kbID).Scan(&stats.documents); err != nil {
		return stats, apperrors.Wrap(apperrors.CodeInternal, "failed to count knowledge documents", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_chunks WHERE tenant_id = ? AND kb_id = ?`, tenantID, kbID).Scan(&stats.chunks); err != nil {
		return stats, apperrors.Wrap(apperrors.CodeInternal, "failed to count knowledge chunks", err)
	}
	return stats, nil
}

func (s *SQLiteStore) ensureBaseStats(ctx context.Context, document knowledge.Document) error {
	base, err := s.GetKnowledgeBase(ctx, document.TenantID.String(), document.KBID.String())
	if err != nil {
		base = knowledge.Base{
			ID:       document.KBID,
			TenantID: document.TenantID,
			Name:     document.KBID.String(),
			Status:   knowledge.BaseStatusEnabled,
			Version:  "v1",
		}
	}
	base.UpdatedAt = time.Now().UTC()
	return s.SaveKnowledgeBase(ctx, base)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanKnowledgeBase(row rowScanner) (knowledge.Base, error) {
	var base knowledge.Base
	var status, metadataRaw, updatedAt string
	if err := row.Scan(
		&base.TenantID,
		&base.ID,
		&base.Name,
		&base.Description,
		&status,
		&base.EmbeddingModel,
		&base.DocumentCount,
		&base.ChunkCount,
		&metadataRaw,
		&base.Version,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return knowledge.Base{}, apperrors.New(apperrors.CodeNotFound, "knowledge base not found")
		}
		return knowledge.Base{}, apperrors.Wrap(apperrors.CodeInternal, "failed to scan knowledge base", err)
	}
	base.Status = knowledge.BaseStatus(status)
	base.Metadata = map[string]any{}
	if err := json.Unmarshal([]byte(metadataRaw), &base.Metadata); err != nil {
		return knowledge.Base{}, apperrors.Wrap(apperrors.CodeInternal, "failed to decode knowledge base metadata", err)
	}
	base.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return base, nil
}

func scanKnowledgeDocument(row rowScanner) (knowledge.Document, error) {
	var document knowledge.Document
	var metadataRaw string
	if err := row.Scan(&document.TenantID, &document.KBID, &document.ID, &document.Title, &document.Text, &metadataRaw, &document.Version); err != nil {
		return knowledge.Document{}, apperrors.Wrap(apperrors.CodeInternal, "failed to scan knowledge document", err)
	}
	document.Metadata = map[string]any{}
	if err := json.Unmarshal([]byte(metadataRaw), &document.Metadata); err != nil {
		return knowledge.Document{}, apperrors.Wrap(apperrors.CodeInternal, "failed to decode knowledge document metadata", err)
	}
	return document, nil
}

func encodeJSON(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to encode metadata json", err)
	}
	return string(data), nil
}
