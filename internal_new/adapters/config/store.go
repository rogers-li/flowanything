package configadapter

import (
	"context"
	"fmt"
	"time"

	coreconfig "flow-anything/core/config"
)

// BundleStore is the persistence boundary for config-as-code bundles.
//
// Runtime services should depend on this interface rather than a specific
// database or file format. Editors can save a bundle; runtimes can load a
// published bundle snapshot by id.
type BundleStore interface {
	LoadBundle(ctx context.Context, id string) (coreconfig.BundleSpec, error)
	SaveBundle(ctx context.Context, bundle coreconfig.BundleSpec) error
	DeleteBundle(ctx context.Context, id string) error
	ListBundles(ctx context.Context) ([]BundleSummary, error)
}

type BundleStores struct {
	Drafts   BundleStore
	Previews BundleStore
	Releases BundleStore
}

func (s BundleStores) Validate() error {
	if s.Drafts == nil {
		return fmt.Errorf("draft bundle store is required")
	}
	if s.Previews == nil {
		return fmt.Errorf("preview bundle store is required")
	}
	if s.Releases == nil {
		return fmt.Errorf("release bundle store is required")
	}
	return nil
}

type BundleSummary struct {
	ID             string
	Name           string
	Version        string
	Lifecycle      coreconfig.BundleLifecycle
	SourceBundleID string
	ContentHash    string
	UpdatedAt      time.Time
}

func validateBundleForStore(bundle coreconfig.BundleSpec) error {
	if bundle.ID == "" {
		return fmt.Errorf("bundle id is required")
	}
	if bundle.Version == "" {
		return fmt.Errorf("bundle version is required")
	}
	return nil
}

func summaryFromBundle(bundle coreconfig.BundleSpec, updatedAt time.Time) BundleSummary {
	return BundleSummary{
		ID:             bundle.ID,
		Name:           bundle.Name,
		Version:        bundle.Version,
		Lifecycle:      metadataLifecycle(bundle),
		SourceBundleID: metadataString(bundle, coreconfig.BundleMetadataSourceBundleID),
		ContentHash:    metadataString(bundle, coreconfig.BundleMetadataContentHash),
		UpdatedAt:      updatedAt,
	}
}

func metadataLifecycle(bundle coreconfig.BundleSpec) coreconfig.BundleLifecycle {
	lifecycle := coreconfig.BundleLifecycle(metadataString(bundle, coreconfig.BundleMetadataLifecycle))
	if lifecycle == "" {
		return coreconfig.BundleLifecycleDraft
	}
	return lifecycle
}

func metadataString(bundle coreconfig.BundleSpec, key string) string {
	if bundle.Metadata == nil {
		return ""
	}
	value, ok := bundle.Metadata[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
