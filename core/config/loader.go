package config

import (
	"encoding/json"
	"io"
)

func LoadBundleJSON(reader io.Reader) (BundleSpec, error) {
	var bundle BundleSpec
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return BundleSpec{}, err
	}
	return bundle, nil
}

func WriteBundleJSON(writer io.Writer, bundle BundleSpec) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(bundle)
}
