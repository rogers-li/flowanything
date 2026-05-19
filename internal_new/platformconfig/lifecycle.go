package platformconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	coreconfig "flow-anything/core/config"
)

type BundleSnapshotInfo struct {
	BundleID       string                        `json:"bundle_id"`
	SourceBundleID string                        `json:"source_bundle_id,omitempty"`
	Lifecycle      coreconfig.BundleLifecycle    `json:"lifecycle"`
	Version        string                        `json:"version"`
	ContentHash    string                        `json:"content_hash"`
	Entrypoint     coreconfig.BundleEntrypoint   `json:"entrypoint,omitempty"`
	Entrypoints    []coreconfig.BundleEntrypoint `json:"entrypoints,omitempty"`
	Dependencies   []coreconfig.DependencyEdge   `json:"dependencies,omitempty"`
	CreatedAt      time.Time                     `json:"created_at"`
}

func preparePreviewBundle(source coreconfig.BundleSpec, entrypoint coreconfig.BundleEntrypoint, now time.Time) (coreconfig.BundleSpec, BundleSnapshotInfo, error) {
	normalized := normalizeDraftBundle(source)
	if err := coreconfig.ValidateBundle(normalized); err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	index, err := coreconfig.BuildIndex(normalized)
	if err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	if err := validateEntrypoint(index, entrypoint); err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	if _, err := coreconfig.CompileRuntimeCatalog(normalized); err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}

	hash, err := bundleContentHash(normalized)
	if err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	preview := normalized
	preview.ID = snapshotID("preview", normalized.ID, entrypoint.ID, hash)
	preview.Metadata = snapshotMetadata(normalized, coreconfig.BundleLifecyclePreview, hash, now, entrypoint)
	info := snapshotInfo(preview)
	return preview, info, nil
}

func prepareReleaseBundle(source coreconfig.BundleSpec, now time.Time) (coreconfig.BundleSpec, BundleSnapshotInfo, error) {
	normalized := normalizeDraftBundle(source)
	if err := coreconfig.ValidateBundle(normalized); err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	if _, err := coreconfig.CompileRuntimeCatalog(normalized); err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}

	hash, err := bundleContentHash(normalized)
	if err != nil {
		return coreconfig.BundleSpec{}, BundleSnapshotInfo{}, err
	}
	release := normalized
	release.ID = snapshotID("release", normalized.ID, "", hash)
	release.Metadata = snapshotMetadata(normalized, coreconfig.BundleLifecycleRelease, hash, now, coreconfig.BundleEntrypoint{})
	info := snapshotInfo(release)
	return release, info, nil
}

func snapshotMetadata(bundle coreconfig.BundleSpec, lifecycle coreconfig.BundleLifecycle, hash string, now time.Time, entrypoint coreconfig.BundleEntrypoint) map[string]any {
	metadata := cloneMetadata(bundle.Metadata)
	metadata[coreconfig.BundleMetadataLifecycle] = string(lifecycle)
	metadata[coreconfig.BundleMetadataSourceBundleID] = bundle.ID
	metadata[coreconfig.BundleMetadataContentHash] = hash
	metadata[coreconfig.BundleMetadataCreatedAt] = now.UTC().Format(time.RFC3339Nano)
	metadata[coreconfig.BundleMetadataDependencyGraph] = coreconfig.ExtractDependencyEdges(bundle)
	entrypoints := collectEntrypoints(bundle)
	if len(entrypoints) > 0 {
		metadata[coreconfig.BundleMetadataEntrypoints] = entrypoints
	}
	if entrypoint.ID != "" {
		metadata[coreconfig.BundleMetadataEntrypoint] = entrypoint
	}
	return metadata
}

func snapshotInfo(bundle coreconfig.BundleSpec) BundleSnapshotInfo {
	metadata := bundle.Metadata
	info := BundleSnapshotInfo{
		BundleID:       bundle.ID,
		SourceBundleID: metadataString(metadata, coreconfig.BundleMetadataSourceBundleID),
		Lifecycle:      coreconfig.BundleLifecycle(metadataString(metadata, coreconfig.BundleMetadataLifecycle)),
		Version:        bundle.Version,
		ContentHash:    metadataString(metadata, coreconfig.BundleMetadataContentHash),
		Dependencies:   coreconfig.ExtractDependencyEdges(bundle),
	}
	if info.Lifecycle == "" {
		info.Lifecycle = coreconfig.BundleLifecycleDraft
	}
	if createdAt := metadataString(metadata, coreconfig.BundleMetadataCreatedAt); createdAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			info.CreatedAt = parsed
		}
	}
	if info.CreatedAt.IsZero() {
		info.CreatedAt = time.Time{}
	}
	info.Entrypoint = metadataEntrypoint(metadata, coreconfig.BundleMetadataEntrypoint)
	info.Entrypoints = collectEntrypoints(bundle)
	return info
}

func bundleContentHash(bundle coreconfig.BundleSpec) (string, error) {
	hashInput := normalizeDraftBundle(bundle)
	hashInput.Metadata = cloneMetadata(hashInput.Metadata)
	for _, key := range []string{
		coreconfig.BundleMetadataLifecycle,
		coreconfig.BundleMetadataSourceBundleID,
		coreconfig.BundleMetadataContentHash,
		coreconfig.BundleMetadataCreatedAt,
		coreconfig.BundleMetadataEntrypoint,
		coreconfig.BundleMetadataEntrypoints,
		coreconfig.BundleMetadataDependencyGraph,
	} {
		delete(hashInput.Metadata, key)
	}
	encoded, err := json.Marshal(hashInput)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validateEntrypoint(index coreconfig.Index, entrypoint coreconfig.BundleEntrypoint) error {
	if entrypoint.ID == "" {
		return fmt.Errorf("entrypoint id is required")
	}
	switch entrypoint.Kind {
	case coreconfig.ResourceAgent, coreconfig.ResourceWorkflow:
	default:
		return fmt.Errorf("unsupported entrypoint kind %q", entrypoint.Kind)
	}
	if !index.Exists(coreconfig.ResourceRef{Kind: entrypoint.Kind, ID: entrypoint.ID}) {
		return fmt.Errorf("entrypoint %s %q not found", entrypoint.Kind, entrypoint.ID)
	}
	return nil
}

func collectEntrypoints(bundle coreconfig.BundleSpec) []coreconfig.BundleEntrypoint {
	out := []coreconfig.BundleEntrypoint{}
	for _, agent := range bundle.Resources.Agents {
		if !agent.Disabled {
			out = append(out, coreconfig.BundleEntrypoint{Kind: coreconfig.ResourceAgent, ID: agent.ID})
		}
	}
	for _, workflow := range bundle.Resources.Workflows {
		if !workflow.Disabled {
			out = append(out, coreconfig.BundleEntrypoint{Kind: coreconfig.ResourceWorkflow, ID: workflow.ID})
		}
	}
	return out
}

func metadataEntrypoint(metadata map[string]any, key string) coreconfig.BundleEntrypoint {
	value, ok := metadata[key]
	if !ok || value == nil {
		return coreconfig.BundleEntrypoint{}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return coreconfig.BundleEntrypoint{}
	}
	var entrypoint coreconfig.BundleEntrypoint
	if err := json.Unmarshal(encoded, &entrypoint); err != nil {
		return coreconfig.BundleEntrypoint{}
	}
	return entrypoint
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func snapshotID(prefix string, sourceID string, entrypointID string, hash string) string {
	parts := []string{prefix, sanitizeID(sourceID)}
	if entrypointID != "" {
		parts = append(parts, sanitizeID(entrypointID))
	}
	if len(hash) > 12 {
		hash = hash[:12]
	}
	parts = append(parts, hash)
	return strings.Join(parts, "_")
}

var unsafeIDChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeID(id string) string {
	id = unsafeIDChars.ReplaceAllString(id, "_")
	id = strings.Trim(id, "_")
	if id == "" {
		return "bundle"
	}
	return id
}
