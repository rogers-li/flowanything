package expression

import "errors"

var (
	ErrPathNotFound      = errors.New("path not found")
	ErrInvalidPath       = errors.New("invalid path")
	ErrUnknownSourceType = errors.New("unknown source type")
)

type Context interface {
	Read(path string) (any, bool)
	Write(path string, value any) error
}

type SourceType string

const (
	SourceContext    SourceType = "context"
	SourceConst      SourceType = "const"
	SourceNodeOutput SourceType = "node_output"
)

type ValueSource struct {
	Type  SourceType `json:"type"`
	Path  string     `json:"path,omitempty"`
	Value any        `json:"value,omitempty"`
}

type FieldBinding struct {
	Field       string      `json:"field"`
	Source      ValueSource `json:"source"`
	Enabled     bool        `json:"enabled"`
	Description string      `json:"description,omitempty"`
}

type ContextWrite struct {
	Target      string      `json:"target"`
	Source      ValueSource `json:"source"`
	Enabled     bool        `json:"enabled"`
	Description string      `json:"description,omitempty"`
}
