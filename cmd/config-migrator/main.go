package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	coreconfig "flow-anything/core/config"
	"flow-anything/internal_new/migration/legacy"
)

func main() {
	var source string
	var output string
	var tenantID string
	var bundleID string
	var name string
	var version string
	var lifecycle string
	var reportOnly bool
	flag.StringVar(&source, "source", "flow-anything.db", "legacy sqlite registry path")
	flag.StringVar(&output, "output", "configs/local/workspace.migrated.bundle.json", "output bundle json path")
	flag.StringVar(&tenantID, "tenant", "tenant_1", "legacy tenant id to migrate")
	flag.StringVar(&bundleID, "bundle-id", "workspace_migrated", "generated bundle id")
	flag.StringVar(&name, "name", "Migrated AI Platform Workspace", "generated bundle name")
	flag.StringVar(&version, "version", "migrated", "generated bundle version")
	flag.StringVar(&lifecycle, "lifecycle", "draft", "generated bundle lifecycle: draft, preview, or release")
	flag.BoolVar(&reportOnly, "report-only", false, "print migration report without writing bundle")
	flag.Parse()

	result, err := legacy.MigrateP0(context.Background(), legacy.Options{
		SourcePath: source,
		TenantID:   tenantID,
		BundleID:   bundleID,
		Name:       name,
		Version:    version,
		Lifecycle:  lifecycle,
	})
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	printReport(result.Report)
	if reportOnly {
		return
	}
	if err := writeBundle(output, result.Bundle); err != nil {
		log.Fatalf("write bundle failed: %v", err)
	}
	fmt.Printf("bundle written: %s\n", output)
}

func writeBundle(path string, bundle coreconfig.BundleSpec) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return coreconfig.WriteBundleJSON(file, bundle)
}

func printReport(report legacy.Report) {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Printf("migration report: %+v\n", report)
		return
	}
	fmt.Printf("migration report:\n%s\n", string(payload))
}
