package mcp

import (
	"context"
	"strings"
	"time"

	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type Caller interface {
	Call(ctx context.Context, req tool.MCPCallRequest) (tool.BackendResult, error)
}

type Adapter struct {
	caller Caller
}

func New(caller Caller) *Adapter {
	return &Adapter{caller: caller}
}

func (a *Adapter) Supports(kind tool.ImplementationType) bool {
	return kind == tool.ImplementationMCP
}

func (a *Adapter) Execute(ctx context.Context, spec tool.Spec, call tool.Call) (tool.Result, error) {
	startedAt := time.Now().UTC()
	if a.caller == nil {
		return tool.Result{}, apperrors.New(apperrors.CodeUnavailable, "mcp caller is not configured")
	}
	if spec.Binding.MCPServerID.Empty() || strings.TrimSpace(spec.Binding.MCPToolName) == "" {
		return tool.Result{}, apperrors.New(apperrors.CodeInvalidArgument, "mcp server and tool binding are required")
	}

	resp, err := a.caller.Call(ctx, tool.MCPCallRequest{
		ID:        id.New("mcpcall"),
		TenantID:  call.TenantID,
		ToolID:    spec.ID,
		ServerID:  spec.Binding.MCPServerID,
		ServerURL: strings.TrimSpace(spec.Binding.MCPServerURL),
		Transport: strings.TrimSpace(spec.Binding.MCPTransport),
		Headers:   spec.Binding.MCPHeaders,
		ToolName:  strings.TrimSpace(spec.Binding.MCPToolName),
		Args:      call.Args,
		TraceID:   call.TraceID,
	})
	if err != nil {
		return tool.Result{}, err
	}

	return backendResultToToolResult(call.ID, spec.ID, startedAt, resp), nil
}

func backendResultToToolResult(callID id.ID, toolID id.ID, fallbackStartedAt time.Time, resp tool.BackendResult) tool.Result {
	startedAt := resp.StartedAt
	if startedAt.IsZero() {
		startedAt = fallbackStartedAt
	}
	finishedAt := resp.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}

	return tool.Result{
		CallID:      callID,
		ToolID:      toolID,
		Success:     resp.Success,
		Data:        resp.Data,
		ErrorCode:   resp.ErrorCode,
		ErrorReason: resp.ErrorReason,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
	}
}
