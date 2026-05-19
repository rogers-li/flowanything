package configadapter

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	coreconfig "flow-anything/core/config"
)

type MemoryBundleStore struct {
	mu      sync.Mutex
	bundles map[string]memoryBundleRecord
	nowFn   func() time.Time
}

type memoryBundleRecord struct {
	bundle    coreconfig.BundleSpec
	updatedAt time.Time
}

func NewMemoryBundleStore() *MemoryBundleStore {
	return &MemoryBundleStore{
		bundles: map[string]memoryBundleRecord{},
		nowFn:   time.Now,
	}
}

func NewMemoryBundleStores() BundleStores {
	return BundleStores{
		Drafts:   NewMemoryBundleStore(),
		Previews: NewMemoryBundleStore(),
		Releases: NewMemoryBundleStore(),
	}
}

func (s *MemoryBundleStore) LoadBundle(_ context.Context, id string) (coreconfig.BundleSpec, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.bundles[id]
	if !ok {
		return coreconfig.BundleSpec{}, fmt.Errorf("bundle %q not found", id)
	}
	return record.bundle, nil
}

func (s *MemoryBundleStore) SaveBundle(_ context.Context, bundle coreconfig.BundleSpec) error {
	if err := validateBundleForStore(bundle); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bundles[bundle.ID] = memoryBundleRecord{bundle: bundle, updatedAt: s.nowFn()}
	return nil
}

func (s *MemoryBundleStore) DeleteBundle(_ context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("bundle id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bundles[id]; !ok {
		return fmt.Errorf("bundle %q not found", id)
	}
	delete(s.bundles, id)
	return nil
}

func (s *MemoryBundleStore) ListBundles(_ context.Context) ([]BundleSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]BundleSummary, 0, len(s.bundles))
	for _, record := range s.bundles {
		out = append(out, summaryFromBundle(record.bundle, record.updatedAt))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
