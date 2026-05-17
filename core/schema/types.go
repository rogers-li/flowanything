package schema

type FieldType string

const (
	TypeAny     FieldType = "any"
	TypeString  FieldType = "string"
	TypeNumber  FieldType = "number"
	TypeInteger FieldType = "integer"
	TypeBoolean FieldType = "boolean"
	TypeObject  FieldType = "object"
	TypeArray   FieldType = "array"
)

type Field struct {
	Name        string         `json:"name"`
	Type        FieldType      `json:"type"`
	Description string         `json:"description"`
	Required    bool           `json:"required"`
	Children    []Field        `json:"children,omitempty"`
	Examples    []any          `json:"examples,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Schema []Field

type FieldPath struct {
	Path        string    `json:"path"`
	Field       Field     `json:"field"`
	Depth       int       `json:"depth"`
	ParentPath  string    `json:"parent_path"`
	ChildFields []Field   `json:"child_fields,omitempty"`
	Type        FieldType `json:"type"`
}

type ValidationError struct {
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

type ValidationErrors []ValidationError

func (e *ValidationErrors) Add(path, message string) {
	*e = append(*e, ValidationError{Path: path, Message: message})
}

func (e ValidationErrors) Error() string {
	out := ""
	for i, item := range e {
		if i > 0 {
			out += "; "
		}
		out += item.Error()
	}
	return out
}

func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}
