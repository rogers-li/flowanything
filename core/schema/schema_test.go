package schema

import (
	"strings"
	"testing"
)

func TestValidateValueSupportsNestedObjectsAndArrays(t *testing.T) {
	fields := Schema{
		{Name: "query", Type: TypeString, Required: true},
		{
			Name: "options",
			Type: TypeObject,
			Children: []Field{
				{Name: "limit", Type: TypeInteger, Required: true},
			},
		},
		{
			Name: "results",
			Type: TypeArray,
			Children: []Field{
				{Name: "title", Type: TypeString, Required: true},
			},
		},
	}

	err := ValidateValue(fields, map[string]any{
		"query":   "AI news",
		"options": map[string]any{"limit": 5},
		"results": []any{
			map[string]any{"title": "one"},
			map[string]any{"title": "two"},
		},
	})
	if err != nil {
		t.Fatalf("expected valid value: %v", err)
	}

	err = ValidateValue(fields, map[string]any{
		"options": map[string]any{"limit": "five"},
		"results": []any{map[string]any{}},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "$.query") || !strings.Contains(err.Error(), "$.options.limit") || !strings.Contains(err.Error(), "$.results[0].title") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestDescribeAndFlattenUseStablePaths(t *testing.T) {
	fields := Schema{{
		Name: "document",
		Type: TypeObject,
		Children: []Field{
			{Name: "title", Type: TypeString, Required: true, Description: "Document title"},
		},
	}}

	flattened := Flatten(fields)
	if len(flattened) != 2 {
		t.Fatalf("unexpected flatten result: %#v", flattened)
	}
	if flattened[1].Path != "$.document.title" {
		t.Fatalf("unexpected nested path: %#v", flattened[1])
	}
	description := Describe(fields)
	if !strings.Contains(description, "$.document.title") || !strings.Contains(description, "Document title") {
		t.Fatalf("unexpected description: %s", description)
	}
}
