package expression

import "testing"

func TestBuildObjectAndApplyContextWrites(t *testing.T) {
	ctx := NewMapContext(map[string]any{
		"flow_input": map[string]any{
			"user_request": "search AI news",
		},
		"variables": map[string]any{
			"limit": 3,
		},
	})

	input, err := BuildObject(ctx, []FieldBinding{
		{
			Field:   "query.text",
			Enabled: true,
			Source:  ValueSource{Type: SourceContext, Path: "$.flow_input.user_request"},
		},
		{
			Field:   "query.limit",
			Enabled: true,
			Source:  ValueSource{Type: SourceContext, Path: "$.variables.limit"},
		},
		{
			Field:   "format",
			Enabled: true,
			Source:  ValueSource{Type: SourceConst, Value: "markdown"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadPath(input, "$.query.text"); got != "search AI news" {
		t.Fatalf("unexpected mapped input: %#v", input)
	}
	if got, _ := ReadPath(input, "$.format"); got != "markdown" {
		t.Fatalf("unexpected const input: %#v", input)
	}

	err = ApplyContextWrites(ctx, map[string]any{
		"answer": map[string]any{"text": "done"},
	}, []ContextWrite{{
		Target:  "$.flow_output.return_message",
		Enabled: true,
		Source:  ValueSource{Type: SourceNodeOutput, Path: "$.answer.text"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := ctx.Read("$.flow_output.return_message"); got != "done" {
		t.Fatalf("unexpected context write: %#v", ctx.Data)
	}
}

func TestResolveValueReportsMissingPath(t *testing.T) {
	ctx := NewMapContext(map[string]any{})
	if _, err := ResolveValue(ctx, nil, ValueSource{Type: SourceContext, Path: "$.missing"}); err == nil {
		t.Fatal("expected missing path error")
	}
}
