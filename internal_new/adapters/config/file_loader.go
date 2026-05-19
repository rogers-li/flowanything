package configadapter

import (
	"fmt"
	"os"

	coreconfig "flow-anything/core/config"
)

// LoadBundleFile loads the config-as-code bundle used by the new runtime host.
func LoadBundleFile(path string) (coreconfig.BundleSpec, error) {
	if path == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("bundle path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	defer file.Close()
	return coreconfig.LoadBundleJSON(file)
}

// SaveBundleFile writes a normalized bundle to disk. It is primarily useful for
// editor/admin workflows and local tests.
func SaveBundleFile(path string, bundle coreconfig.BundleSpec) error {
	if path == "" {
		return fmt.Errorf("bundle path is required")
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return coreconfig.WriteBundleJSON(file, bundle)
}
