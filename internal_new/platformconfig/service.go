package platformconfig

import (
	"context"
	"errors"
	"fmt"
	"time"

	coreconfig "flow-anything/core/config"
	configadapter "flow-anything/internal_new/adapters/config"
)

type Diagnostic struct {
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid       bool         `json:"valid"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// Service owns Bundle authoring operations for the admin console.
//
// Saving a bundle is draft-friendly and only requires store-level identity
// fields. Validation and publication are explicit operations so editors can
// persist incomplete work without pretending it is runnable.
type Service struct {
	drafts    configadapter.BundleStore
	previews  configadapter.BundleStore
	releases  configadapter.BundleStore
	publisher Publisher
}

func NewService(store configadapter.BundleStore) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("bundle store is required")
	}
	return NewServiceWithStores(configadapter.BundleStores{
		Drafts:   store,
		Previews: store,
		Releases: store,
	})
}

func NewServiceWithStores(stores configadapter.BundleStores) (*Service, error) {
	if err := stores.Validate(); err != nil {
		return nil, err
	}
	return &Service{
		drafts:    stores.Drafts,
		previews:  stores.Previews,
		releases:  stores.Releases,
		publisher: NewPublisher(stores.Releases),
	}, nil
}

func (s *Service) ListBundles(ctx context.Context) ([]configadapter.BundleSummary, error) {
	return s.drafts.ListBundles(ctx)
}

func (s *Service) GetBundle(ctx context.Context, id string) (coreconfig.BundleSpec, error) {
	return s.drafts.LoadBundle(ctx, id)
}

func (s *Service) ListPreviewBundles(ctx context.Context) ([]configadapter.BundleSummary, error) {
	return s.previews.ListBundles(ctx)
}

func (s *Service) GetPreviewBundle(ctx context.Context, id string) (coreconfig.BundleSpec, error) {
	return s.previews.LoadBundle(ctx, id)
}

func (s *Service) ListReleaseBundles(ctx context.Context) ([]configadapter.BundleSummary, error) {
	return s.releases.ListBundles(ctx)
}

func (s *Service) GetReleaseBundle(ctx context.Context, id string) (coreconfig.BundleSpec, error) {
	bundle, err := s.releases.LoadBundle(ctx, id)
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	if lifecycleOf(bundle) != coreconfig.BundleLifecycleRelease {
		return coreconfig.BundleSpec{}, fmt.Errorf("bundle %q is %q, not release", id, lifecycleOf(bundle))
	}
	return bundle, nil
}

func (s *Service) SaveBundle(ctx context.Context, bundle coreconfig.BundleSpec) (coreconfig.BundleSpec, error) {
	normalized := normalizeDraftBundle(bundle)
	if err := s.drafts.SaveBundle(ctx, normalized); err != nil {
		return coreconfig.BundleSpec{}, err
	}
	return normalized, nil
}

func (s *Service) DeleteBundle(ctx context.Context, id string) error {
	return s.drafts.DeleteBundle(ctx, id)
}

func (s *Service) ValidateBundle(bundle coreconfig.BundleSpec) ValidationResult {
	normalized := normalizeDraftBundle(bundle)
	if err := coreconfig.ValidateBundle(normalized); err != nil {
		return ValidationResult{Valid: false, Diagnostics: diagnosticsFromError(err)}
	}
	if _, err := coreconfig.CompileRuntimeCatalog(normalized); err != nil {
		return ValidationResult{Valid: false, Diagnostics: diagnosticsFromError(err)}
	}
	return ValidationResult{Valid: true}
}

func (s *Service) ValidateStoredBundle(ctx context.Context, id string) (ValidationResult, error) {
	bundle, err := s.drafts.LoadBundle(ctx, id)
	if err != nil {
		return ValidationResult{}, err
	}
	return s.ValidateBundle(bundle), nil
}

func (s *Service) PublishBundle(ctx context.Context, id string) (PublishResult, error) {
	bundle, err := s.drafts.LoadBundle(ctx, id)
	if err != nil {
		return PublishResult{}, err
	}
	return s.publisher.Publish(ctx, bundle)
}

func (s *Service) BuildPreviewBundle(ctx context.Context, id string, entrypoint coreconfig.BundleEntrypoint) (coreconfig.BundleSpec, BundleSnapshotInfo, error) {
	bundle, err := s.drafts.LoadBundle(ctx, id)
	if err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	preview, info, err := preparePreviewBundle(bundle, entrypoint, time.Now())
	if err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	if err := s.previews.SaveBundle(ctx, preview); err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	return preview, info, nil
}

func (s *Service) BundleSnapshotInfo(ctx context.Context, id string) (BundleSnapshotInfo, error) {
	bundle, err := s.drafts.LoadBundle(ctx, id)
	if err != nil {
		return BundleSnapshotInfo{}, err
	}
	return snapshotInfo(bundle), nil
}

func (s *Service) loadBundle(ctx context.Context, id string, lifecycle coreconfig.BundleLifecycle) (coreconfig.BundleSpec, error) {
	switch lifecycle {
	case "", coreconfig.BundleLifecycleDraft:
		return s.drafts.LoadBundle(ctx, id)
	case coreconfig.BundleLifecyclePreview:
		return s.previews.LoadBundle(ctx, id)
	case coreconfig.BundleLifecycleRelease:
		return s.GetReleaseBundle(ctx, id)
	default:
		return coreconfig.BundleSpec{}, fmt.Errorf("unsupported bundle lifecycle %q", lifecycle)
	}
}

func lifecycleOf(bundle coreconfig.BundleSpec) coreconfig.BundleLifecycle {
	if bundle.Metadata == nil {
		return ""
	}
	if value, ok := bundle.Metadata[coreconfig.BundleMetadataLifecycle].(string); ok {
		return coreconfig.BundleLifecycle(value)
	}
	return ""
}

func normalizeDraftBundle(bundle coreconfig.BundleSpec) coreconfig.BundleSpec {
	if bundle.SchemaVersion == "" {
		bundle.SchemaVersion = coreconfig.SchemaVersionV1
	}
	if bundle.Metadata == nil {
		bundle.Metadata = map[string]any{}
	}
	if _, ok := bundle.Metadata[coreconfig.BundleMetadataLifecycle]; !ok {
		bundle.Metadata[coreconfig.BundleMetadataLifecycle] = string(coreconfig.BundleLifecycleDraft)
	}
	return coreconfig.NormalizeBundle(bundle)
}

func diagnosticsFromError(err error) []Diagnostic {
	if err == nil {
		return nil
	}
	var validationErrors coreconfig.ValidationErrors
	if errors.As(err, &validationErrors) {
		out := make([]Diagnostic, 0, len(validationErrors))
		for _, item := range validationErrors {
			out = append(out, Diagnostic{Path: item.Path, Message: item.Message})
		}
		return out
	}
	return []Diagnostic{{Message: err.Error()}}
}
