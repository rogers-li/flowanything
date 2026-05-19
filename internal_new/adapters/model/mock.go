package modeladapter

import (
	"fmt"

	"flow-anything/core/agentcore"
)

// MockClient is a deterministic model adapter for local tests and offline
// runtime smoke checks.
type MockClient struct {
	Content string
}

func (c MockClient) Chat(_ agentcore.Context, req agentcore.ModelRequest) (agentcore.ModelResponse, error) {
	content := c.Content
	if content == "" {
		content = fmt.Sprintf("mock response from %s", req.Model.Model)
	}
	return agentcore.ModelResponse{
		Message: agentcore.Message{Role: "assistant", Content: content},
		Raw: map[string]any{
			"provider":      "mock",
			"message_count": len(req.Messages),
			"tool_count":    len(req.Tools),
		},
		Provider: "mock",
		Model:    req.Model.Model,
		TraceID:  req.TraceID,
	}, nil
}
