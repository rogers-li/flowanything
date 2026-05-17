package infrastructure

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/mcpclient"
)

type HTTPMCPCaller struct {
	client        *httpclient.Client
	logger        *slog.Logger
	directTimeout time.Duration
}

func NewHTTPMCPCaller(mcpGatewayBaseURL string, logger *slog.Logger, timeout time.Duration) *HTTPMCPCaller {
	if timeout <= 0 {
		timeout = 75 * time.Second
	}
	return &HTTPMCPCaller{
		client:        httpclient.New(mcpGatewayBaseURL, timeout),
		logger:        logger,
		directTimeout: timeout,
	}
}

func (c *HTTPMCPCaller) Call(ctx context.Context, req tool.MCPCallRequest) (tool.BackendResult, error) {
	if req.ServerURL != "" {
		return c.callDirect(ctx, req)
	}

	var result tool.BackendResult
	if err := c.client.PostJSON(ctx, "/v1/mcp/tools/call", req, &result); err != nil {
		return tool.BackendResult{}, err
	}

	return result, nil
}

func (c *HTTPMCPCaller) callDirect(ctx context.Context, req tool.MCPCallRequest) (tool.BackendResult, error) {
	startedAt := time.Now().UTC()
	timeout := c.directTimeout
	if timeout <= 0 {
		timeout = 75 * time.Second
	}
	if c.logger != nil {
		c.logger.Info("mcp direct tool call started",
			"mcp_call_id", req.ID.String(),
			"tenant_id", req.TenantID.String(),
			"tool_id", req.ToolID.String(),
			"server_id", req.ServerID.String(),
			"server_url", sanitizedURL(req.ServerURL),
			"transport", req.Transport,
			"mcp_tool_name", req.ToolName,
			"arg_keys", mapKeys(req.Args),
			"header_keys", mapStringKeys(req.Headers),
			"timeout_ms", timeout.Milliseconds(),
			"trace_id", req.TraceID,
		)
	}
	result, err := mcpclient.New(timeout).CallTool(ctx, mcpclient.ServerConfig{
		Name:      req.ServerID.String(),
		URL:       req.ServerURL,
		Transport: req.Transport,
		Headers:   req.Headers,
	}, req.ToolName, req.Args)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("mcp direct tool call failed",
				"mcp_call_id", req.ID.String(),
				"tenant_id", req.TenantID.String(),
				"tool_id", req.ToolID.String(),
				"server_id", req.ServerID.String(),
				"server_url", sanitizedURL(req.ServerURL),
				"transport", req.Transport,
				"mcp_tool_name", req.ToolName,
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"trace_id", req.TraceID,
				"error", err,
			)
		}
		return tool.BackendResult{}, err
	}
	if c.logger != nil {
		c.logger.Info("mcp direct tool call completed",
			"mcp_call_id", req.ID.String(),
			"tenant_id", req.TenantID.String(),
			"tool_id", req.ToolID.String(),
			"server_id", req.ServerID.String(),
			"server_url", sanitizedURL(req.ServerURL),
			"transport", req.Transport,
			"mcp_tool_name", req.ToolName,
			"success", result.Success,
			"error_code", result.ErrorCode,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"trace_id", req.TraceID,
		)
	}
	return tool.BackendResult{
		RequestID:   req.ID,
		Success:     result.Success,
		Data:        result.Data,
		ErrorCode:   result.ErrorCode,
		ErrorReason: result.ErrorReason,
		StartedAt:   startedAt,
		FinishedAt:  time.Now().UTC(),
	}, nil
}

func sanitizedURL(raw string) string {
	if i := strings.Index(raw, "?"); i >= 0 {
		return raw[:i]
	}
	return raw
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func mapStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
