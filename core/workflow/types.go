package workflow

import (
	"time"

	"flow-anything/core/flowengine"
)

// WorkflowDocument is the persisted configuration document shared by editors
// and runtimes. Runtime execution uses Spec directly.
type WorkflowDocument struct {
	ID      string              `json:"id"`
	Spec    flowengine.FlowSpec `json:"spec"`
	UI      UIMetadata          `json:"ui"`
	Publish PublishMetadata     `json:"publish"`
}

// UIMetadata stores editor-only visual metadata.
type UIMetadata struct {
	Viewport ViewportMetadata          `json:"viewport"`
	Nodes    map[string]NodeUIMetadata `json:"nodes"`
	Edges    map[string]EdgeUIMetadata `json:"edges"`
}

type ViewportMetadata struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

type NodeUIMetadata struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Collapsed bool    `json:"collapsed"`
	Color     string  `json:"color"`
}

type EdgeUIMetadata struct {
	Label  string `json:"label"`
	Router string `json:"router"`
}

// PublishMetadata stores product-level publishing status and snapshot identity.
type PublishMetadata struct {
	Status         PublishStatus `json:"status"`
	Revision       int64         `json:"revision"`
	PublishedBy    string        `json:"published_by"`
	PublishedAt    time.Time     `json:"published_at"`
	SnapshotID     string        `json:"snapshot_id"`
	SnapshotHash   string        `json:"snapshot_hash"`
	SourceDocument string        `json:"source_document"`
}

type PublishStatus string

const (
	PublishDraft     PublishStatus = "draft"
	PublishValidated PublishStatus = "validated"
	PublishPublished PublishStatus = "published"
)

// CompiledWorkflow is the immutable executable artifact produced at publish
// time. Spec should remain visually equivalent to WorkflowDocument.Spec.
type CompiledWorkflow struct {
	DocumentID string
	SnapshotID string
	Spec       flowengine.FlowSpec
	Hash       string
	CompiledAt time.Time
}
