package configadapter

import (
	"context"
	"database/sql"
	"testing"
	"time"

	coreconfig "flow-anything/core/config"

	_ "modernc.org/sqlite"
)

func TestMemoryBundleStoreSaveLoadList(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryBundleStore()
	bundle := testBundle("bundle_memory", "v1")

	if err := store.SaveBundle(ctx, bundle); err != nil {
		t.Fatalf("save bundle: %v", err)
	}
	loaded, err := store.LoadBundle(ctx, bundle.ID)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if loaded.ID != bundle.ID || loaded.Version != bundle.Version {
		t.Fatalf("loaded wrong bundle: %#v", loaded)
	}
	summaries, err := store.ListBundles(ctx)
	if err != nil {
		t.Fatalf("list bundles: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != bundle.ID {
		t.Fatalf("unexpected summaries: %#v", summaries)
	}
	if err := store.DeleteBundle(ctx, bundle.ID); err != nil {
		t.Fatalf("delete bundle: %v", err)
	}
	if _, err := store.LoadBundle(ctx, bundle.ID); err == nil {
		t.Fatal("expected deleted bundle to be missing")
	}
}

func TestSQLBundleStoreSaveLoadUpdateList(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store := NewSQLBundleStore(db, SQLDialectSQLite)
	store.NowFn = func() time.Time { return time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC) }
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	bundle := testBundle("bundle_sql", "v1")
	if err := store.SaveBundle(ctx, bundle); err != nil {
		t.Fatalf("save bundle: %v", err)
	}
	bundle.Version = "v2"
	if err := store.SaveBundle(ctx, bundle); err != nil {
		t.Fatalf("update bundle: %v", err)
	}
	loaded, err := store.LoadBundle(ctx, bundle.ID)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if loaded.Version != "v2" {
		t.Fatalf("expected updated version, got %q", loaded.Version)
	}
	summaries, err := store.ListBundles(ctx)
	if err != nil {
		t.Fatalf("list bundles: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Version != "v2" {
		t.Fatalf("unexpected summaries: %#v", summaries)
	}
	if err := store.DeleteBundle(ctx, bundle.ID); err != nil {
		t.Fatalf("delete bundle: %v", err)
	}
	summaries, err = store.ListBundles(ctx)
	if err != nil {
		t.Fatalf("list bundles after delete: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected no summaries after delete, got %#v", summaries)
	}
}

func TestFileBundleStoreSaveLoad(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/bundle.json"
	store := NewFileBundleStore(path)
	bundle := testBundle("bundle_file", "v1")

	if err := store.SaveBundle(ctx, bundle); err != nil {
		t.Fatalf("save bundle: %v", err)
	}
	loaded, err := store.LoadBundle(ctx, "bundle_file")
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if loaded.ID != "bundle_file" {
		t.Fatalf("unexpected loaded bundle: %#v", loaded)
	}
	if err := store.DeleteBundle(ctx, "bundle_file"); err != nil {
		t.Fatalf("delete bundle: %v", err)
	}
	if _, err := store.LoadBundle(ctx, "bundle_file"); err == nil {
		t.Fatal("expected deleted file bundle to be missing")
	}
}

func testBundle(id string, version string) coreconfig.BundleSpec {
	return coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		Kind:          coreconfig.BundleKind,
		ID:            id,
		Name:          "Test Bundle",
		Version:       version,
	}
}
