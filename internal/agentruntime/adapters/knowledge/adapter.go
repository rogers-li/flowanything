package knowledge

import (
	"context"
	"fmt"
	"time"

	knowledgecontract "flow-anything/internal/platform/contracts/knowledge"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type Retriever interface {
	Search(ctx context.Context, query knowledgecontract.Query) (knowledgecontract.Result, error)
}

type Adapter struct {
	retriever Retriever
}

func New(retriever Retriever) *Adapter {
	return &Adapter{retriever: retriever}
}

func (a *Adapter) Supports(kind tool.ImplementationType) bool {
	return kind == tool.ImplementationKnowledge
}

func (a *Adapter) Execute(ctx context.Context, spec tool.Spec, call tool.Call) (tool.Result, error) {
	startedAt := time.Now().UTC()
	if a.retriever == nil {
		return tool.Result{}, apperrors.New(apperrors.CodeUnavailable, "knowledge retriever is not configured")
	}

	queryText := stringArg(call.Args, "query")
	if queryText == "" {
		queryText = stringArg(call.Args, "text")
	}
	if queryText == "" {
		return tool.Result{}, apperrors.New(apperrors.CodeInvalidArgument, "knowledge query is required")
	}

	query := knowledgecontract.Query{
		ID:       id.New("kbq"),
		TenantID: call.TenantID,
		KBIDs:    knowledgeBaseIDs(spec, call.Args),
		Text:     queryText,
		TopK:     intArg(call.Args, "top_k", 5),
		Filters:  mapArg(call.Args, "filters"),
		TraceID:  call.TraceID,
	}
	resp, err := a.retriever.Search(ctx, query)
	if err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		CallID:  call.ID,
		ToolID:  spec.ID,
		Success: true,
		Data: map[string]any{
			"query_id": resp.QueryID,
			"chunks":   resp.Chunks,
		},
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	}, nil
}

func knowledgeBaseIDs(spec tool.Spec, args map[string]any) []id.ID {
	if len(spec.Binding.KnowledgeBaseIDs) > 0 {
		return spec.Binding.KnowledgeBaseIDs
	}

	switch values := args["kb_ids"].(type) {
	case []any:
		result := make([]id.ID, 0, len(values))
		for _, value := range values {
			if text := fmt.Sprint(value); text != "" {
				result = append(result, id.ID(text))
			}
		}
		return result
	case []string:
		result := make([]id.ID, 0, len(values))
		for _, value := range values {
			if value != "" {
				result = append(result, id.ID(value))
			}
		}
		return result
	case []id.ID:
		return values
	default:
		return nil
	}
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func intArg(args map[string]any, key string, fallback int) int {
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}

func mapArg(args map[string]any, key string) map[string]any {
	value, _ := args[key].(map[string]any)
	return value
}
