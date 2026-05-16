package domain

import (
	"flow-anything/internal/platform/kernel/id"
)

type ToolRequest struct {
	ToolID    id.ID
	Args      map[string]any
	Confirmed bool
}

func NewToolRequest(payload map[string]any) (ToolRequest, bool) {
	if payload == nil {
		return ToolRequest{}, false
	}

	rawToolID, ok := payload["tool_id"].(string)
	if !ok || rawToolID == "" {
		return ToolRequest{}, false
	}

	args := map[string]any{}
	if rawArgs, ok := payload["tool_args"].(map[string]any); ok {
		args = rawArgs
	}
	if rawArgs, ok := payload["args"].(map[string]any); ok {
		args = rawArgs
	}
	confirmed, _ := payload["confirmed"].(bool)

	return ToolRequest{
		ToolID:    id.ID(rawToolID),
		Args:      args,
		Confirmed: confirmed,
	}, true
}
