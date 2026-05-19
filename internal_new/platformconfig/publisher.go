package platformconfig

import (
	"context"
	"fmt"
	"time"

	coreconfig "flow-anything/core/config"
	configadapter "flow-anything/internal_new/adapters/config"
)

// ResourceCounts summarizes the published bundle without exposing the full
// runtime catalog to API callers that only need publish metadata.
type ResourceCounts struct {
	Agents              int `json:"agents"`
	Skills              int `json:"skills"`
	Tools               int `json:"tools"`
	Workflows           int `json:"workflows"`
	Connectors          int `json:"connectors"`
	ConnectorOperations int `json:"connector_operations"`
	Models              int `json:"models"`
	KnowledgeBases      int `json:"knowledge_bases"`
	Policies            int `json:"policies"`
}

type PublishResult struct {
	BundleID       string                        `json:"bundle_id"`
	SourceBundleID string                        `json:"source_bundle_id,omitempty"`
	Version        string                        `json:"version"`
	Lifecycle      coreconfig.BundleLifecycle    `json:"lifecycle"`
	ContentHash    string                        `json:"content_hash"`
	Entrypoints    []coreconfig.BundleEntrypoint `json:"entrypoints,omitempty"`
	Counts         ResourceCounts                `json:"counts"`
}

type Publisher struct {
	store configadapter.BundleStore
}

func NewPublisher(store configadapter.BundleStore) Publisher {
	return Publisher{store: store}
}

// Publish validates, compiles, and persists a normalized bundle snapshot.
//
// The compile step is deliberately part of publication. It catches errors that
// validation alone cannot see, such as unsupported duration values in execution
// policy fields.
func (p Publisher) Publish(ctx context.Context, bundle coreconfig.BundleSpec) (PublishResult, error) {
	if p.store == nil {
		return PublishResult{}, fmt.Errorf("bundle store is required")
	}
	release, info, err := prepareReleaseBundle(bundle, time.Now())
	if err != nil {
		return PublishResult{}, err
	}
	if err := p.store.SaveBundle(ctx, release); err != nil {
		return PublishResult{}, err
	}
	return PublishResult{
		BundleID:       release.ID,
		SourceBundleID: info.SourceBundleID,
		Version:        release.Version,
		Lifecycle:      info.Lifecycle,
		ContentHash:    info.ContentHash,
		Entrypoints:    info.Entrypoints,
		Counts:         CountResources(release),
	}, nil
}

func CountResources(bundle coreconfig.BundleSpec) ResourceCounts {
	counts := ResourceCounts{
		Agents:         len(bundle.Resources.Agents),
		Skills:         len(bundle.Resources.Skills),
		Tools:          len(bundle.Resources.Tools),
		Workflows:      len(bundle.Resources.Workflows),
		Connectors:     len(bundle.Resources.Connectors),
		Models:         len(bundle.Resources.Models),
		KnowledgeBases: len(bundle.Resources.KnowledgeBases),
		Policies:       len(bundle.Resources.Policies),
	}
	for _, connector := range bundle.Resources.Connectors {
		counts.ConnectorOperations += len(connector.Operations)
	}
	return counts
}
