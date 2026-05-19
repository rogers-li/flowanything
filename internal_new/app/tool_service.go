package app

import (
	"context"

	"flow-anything/core/runtimecontext"
	"flow-anything/core/tools"
)

type ToolRequest struct {
	CallID       string
	ToolID       string
	Input        map[string]any
	Metadata     map[string]any
	TraceID      string
	TraceContext runtimecontext.TraceContext
}

type ToolResult = tools.ToolResult

// InvokeTool executes one configured tool through core/tools.
func (h *Host) InvokeTool(ctx context.Context, req ToolRequest) (ToolResult, error) {
	return h.toolRuntime.Invoke(ctx, tools.ToolCall{
		CallID:       req.CallID,
		ToolID:       req.ToolID,
		Input:        req.Input,
		Metadata:     req.Metadata,
		TraceID:      req.TraceID,
		TraceContext: req.TraceContext,
	})
}
