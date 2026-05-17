package knowledge

import (
	"time"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type BaseStatus string

const (
	BaseStatusDraft    BaseStatus = "draft"
	BaseStatusEnabled  BaseStatus = "enabled"
	BaseStatusDisabled BaseStatus = "disabled"
)

type Base struct {
	ID             id.ID          `json:"id"`
	TenantID       tenant.ID      `json:"tenant_id"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Status         BaseStatus     `json:"status"`
	EmbeddingModel string         `json:"embedding_model,omitempty"`
	DocumentCount  int            `json:"document_count"`
	ChunkCount     int            `json:"chunk_count"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Version        string         `json:"version"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type Query struct {
	ID       id.ID          `json:"id"`
	TenantID tenant.ID      `json:"tenant_id"`
	KBIDs    []id.ID        `json:"kb_ids,omitempty"`
	Text     string         `json:"text"`
	TopK     int            `json:"top_k"`
	Filters  map[string]any `json:"filters,omitempty"`
	TraceID  string         `json:"trace_id,omitempty"`
}

type Document struct {
	ID       id.ID          `json:"id"`
	TenantID tenant.ID      `json:"tenant_id"`
	KBID     id.ID          `json:"kb_id"`
	Title    string         `json:"title"`
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Version  string         `json:"version"`
}

type Chunk struct {
	ID       id.ID          `json:"id"`
	DocID    id.ID          `json:"doc_id"`
	KBID     id.ID          `json:"kb_id"`
	Text     string         `json:"text"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Result struct {
	QueryID id.ID   `json:"query_id"`
	Chunks  []Chunk `json:"chunks"`
}

type BaseListResponse struct {
	Items []Base `json:"items"`
}

type DocumentListResponse struct {
	Items []Document `json:"items"`
}

type IndexResult struct {
	DocumentID id.ID   `json:"document_id"`
	KBID       id.ID   `json:"kb_id"`
	ChunkCount int     `json:"chunk_count"`
	Chunks     []Chunk `json:"chunks,omitempty"`
}
