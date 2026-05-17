package editor

import (
	"fmt"

	"flow-anything/core/config"
)

// MigrateBundle upgrades a bundle draft to a target config schema version. The
// first protocol version has no structural migrations yet, but keeping this
// entry point lets editors share a single upgrade path when v2 appears.
func MigrateBundle(bundle config.BundleSpec, targetSchemaVersion string) (config.BundleSpec, PatchSet, error) {
	if targetSchemaVersion == "" {
		targetSchemaVersion = config.SchemaVersionV1
	}
	if targetSchemaVersion != config.SchemaVersionV1 {
		return config.BundleSpec{}, PatchSet{}, fmt.Errorf("unsupported target schema version %q", targetSchemaVersion)
	}
	before := bundle
	after := config.NormalizeBundle(bundle)
	after.SchemaVersion = config.SchemaVersionV1
	after.Kind = config.BundleKind
	patch, err := DiffBundles(before, after)
	if err != nil {
		return config.BundleSpec{}, PatchSet{}, err
	}
	return after, patch, nil
}
