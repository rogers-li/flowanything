package editor

import (
	"errors"

	"flow-anything/core/config"
)

var (
	ErrUnsupportedPatchOperation = errors.New("unsupported patch operation")
	ErrInvalidPatchPath          = errors.New("invalid patch path")
	ErrResourceNotFound          = errors.New("resource not found")
	ErrDuplicateResource         = errors.New("duplicate resource")
)

type PatchOperationType string

const (
	PatchAdd     PatchOperationType = "add"
	PatchReplace PatchOperationType = "replace"
	PatchRemove  PatchOperationType = "remove"
)

// PatchOperation is an RFC6902-inspired operation over a BundleSpec JSON
// document. Paths use JSON pointer syntax, for example
// "/resources/workflows/0/spec/nodes/0/name".
type PatchOperation struct {
	Op    PatchOperationType `json:"op"`
	Path  string             `json:"path"`
	Value any                `json:"value,omitempty"`
}

type PatchSet struct {
	Operations []PatchOperation `json:"operations"`
}

type DraftInspection struct {
	Bundle       config.BundleSpec           `json:"bundle"`
	Resources    []config.ResourceDescriptor `json:"resources"`
	Dependencies []config.DependencyEdge     `json:"dependencies"`
	Diagnostics  []config.Diagnostic         `json:"diagnostics"`
	Publishable  bool                        `json:"publishable"`
}

type BindingFilter struct {
	Kinds           []config.ResourceKind `json:"kinds"`
	IncludeDisabled bool                  `json:"include_disabled"`
	ParentID        string                `json:"parent_id"`
}
