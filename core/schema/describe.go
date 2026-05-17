package schema

import (
	"fmt"
	"strings"
)

// Flatten returns every field with a stable dot path. It is useful for UI
// selectors and prompt dependency analysis.
func Flatten(fields Schema) []FieldPath {
	out := []FieldPath{}
	flattenInto(&out, "$", "", 0, fields)
	return out
}

// Describe renders a compact, model-friendly schema description.
func Describe(fields Schema) string {
	if len(fields) == 0 {
		return "(empty schema)"
	}
	var builder strings.Builder
	for _, field := range Flatten(fields) {
		indent := strings.Repeat("  ", field.Depth)
		required := "optional"
		if field.Field.Required {
			required = "required"
		}
		description := strings.TrimSpace(field.Field.Description)
		if description != "" {
			description = " - " + description
		}
		builder.WriteString(fmt.Sprintf("%s- %s (%s, %s)%s\n", indent, field.Path, field.Type, required, description))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func flattenInto(out *[]FieldPath, parentPath, parent string, depth int, fields []Field) {
	for _, field := range fields {
		path := parentPath + "." + field.Name
		if parentPath == "$" {
			path = "$." + field.Name
		}
		entry := FieldPath{
			Path:        path,
			Field:       field,
			Depth:       depth,
			ParentPath:  parent,
			ChildFields: field.Children,
			Type:        normalizeType(field.Type),
		}
		*out = append(*out, entry)
		if len(field.Children) > 0 {
			flattenInto(out, path, path, depth+1, field.Children)
		}
	}
}
