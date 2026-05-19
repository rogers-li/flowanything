package configadapter

import (
	"context"
	"fmt"
	"os"
	"time"

	coreconfig "flow-anything/core/config"
)

// FileBundleStore stores one bundle in one JSON file. It is intentionally small
// and useful for local runtime smoke tests or edge/device runtimes.
type FileBundleStore struct {
	Path string
}

func NewFileBundleStore(path string) FileBundleStore {
	return FileBundleStore{Path: path}
}

func (s FileBundleStore) LoadBundle(_ context.Context, id string) (coreconfig.BundleSpec, error) {
	bundle, err := LoadBundleFile(s.Path)
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	if id != "" && bundle.ID != id {
		return coreconfig.BundleSpec{}, fmt.Errorf("bundle %q not found in file store", id)
	}
	return bundle, nil
}

func (s FileBundleStore) SaveBundle(_ context.Context, bundle coreconfig.BundleSpec) error {
	if err := validateBundleForStore(bundle); err != nil {
		return err
	}
	return SaveBundleFile(s.Path, bundle)
}

func (s FileBundleStore) DeleteBundle(_ context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("bundle id is required")
	}
	bundle, err := LoadBundleFile(s.Path)
	if err != nil {
		return err
	}
	if bundle.ID != id {
		return fmt.Errorf("bundle %q not found in file store", id)
	}
	return os.Remove(s.Path)
}

func (s FileBundleStore) ListBundles(_ context.Context) ([]BundleSummary, error) {
	bundle, err := LoadBundleFile(s.Path)
	if err != nil {
		return nil, err
	}
	updatedAt := time.Time{}
	if stat, err := os.Stat(s.Path); err == nil {
		updatedAt = stat.ModTime()
	}
	return []BundleSummary{summaryFromBundle(bundle, updatedAt)}, nil
}
