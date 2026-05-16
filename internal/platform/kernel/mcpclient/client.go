package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "flow-anything/internal/platform/kernel/errors"
)

const protocolVersion = "2025-03-26"

type ServerConfig struct {
	Name      string
	URL       string
	Transport string
	Headers   map[string]string
}

type Tool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

type CallResult struct {
	Success     bool
	Data        map[string]any
	ErrorCode   string
	ErrorReason string
}

type Client struct {
	httpClient *http.Client
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{httpClient: &http.Client{Timeout: timeout}}
}

func (c *Client) DiscoverTools(ctx context.Context, cfg ServerConfig) ([]Tool, error) {
	sessionID, err := c.initialize(ctx, cfg)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeUnavailable, "mcp initialize failed", err)
	}
	if err := c.notifyInitialized(ctx, cfg, sessionID); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeUnavailable, "mcp initialized notification failed", err)
	}

	var all []Tool
	var cursor string
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		resp, err := c.rpc(ctx, cfg, sessionID, rpcRequest{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "tools/list",
			Params:  params,
		}, true)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.CodeUnavailable, "mcp tools/list failed", err)
		}
		var result toolsListResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to decode mcp tools/list result", err)
		}
		for _, remoteTool := range result.Tools {
			all = append(all, remoteTool.toTool())
		}
		if result.NextCursor == "" {
			return all, nil
		}
		cursor = result.NextCursor
	}
}

func (c *Client) CallTool(ctx context.Context, cfg ServerConfig, toolName string, args map[string]any) (CallResult, error) {
	sessionID, err := c.initialize(ctx, cfg)
	if err != nil {
		return CallResult{}, apperrors.Wrap(apperrors.CodeUnavailable, "mcp initialize failed", err)
	}
	if err := c.notifyInitialized(ctx, cfg, sessionID); err != nil {
		return CallResult{}, apperrors.Wrap(apperrors.CodeUnavailable, "mcp initialized notification failed", err)
	}
	resp, err := c.rpc(ctx, cfg, sessionID, rpcRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}, true)
	if err != nil {
		return CallResult{}, apperrors.Wrap(apperrors.CodeUnavailable, "mcp tools/call failed", err)
	}
	var result toolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return CallResult{}, apperrors.Wrap(apperrors.CodeInternal, "failed to decode mcp tools/call result", err)
	}

	data := map[string]any{
		"content": result.Content,
	}
	if result.StructuredContent != nil {
		data["structured_content"] = result.StructuredContent
	}
	if result.IsError {
		return CallResult{
			Success:     false,
			Data:        data,
			ErrorCode:   "mcp_tool_error",
			ErrorReason: firstTextContent(result.Content),
		}, nil
	}
	return CallResult{Success: true, Data: data}, nil
}

func (c *Client) initialize(ctx context.Context, cfg ServerConfig) (string, error) {
	resp, err := c.rpc(ctx, cfg, "", rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "flow-anything",
				"version": "0.1.0",
			},
		},
	}, true)
	if err != nil {
		return "", err
	}
	return resp.SessionID, nil
}

func (c *Client) notifyInitialized(ctx context.Context, cfg ServerConfig, sessionID string) error {
	_, err := c.rpc(ctx, cfg, sessionID, rpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}, false)
	return err
}

func (c *Client) rpc(ctx context.Context, cfg ServerConfig, sessionID string, req rpcRequest, expectResponse bool) (rpcResponse, error) {
	endpoint, err := normalizeEndpoint(cfg.URL)
	if err != nil {
		return rpcResponse{}, err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return rpcResponse{}, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to encode mcp json-rpc request", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return rpcResponse{}, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to build mcp request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("MCP-Protocol-Version", protocolVersion)
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}
	for name, value := range cfg.Headers {
		if strings.TrimSpace(name) != "" {
			httpReq.Header.Set(name, value)
		}
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return rpcResponse{}, apperrors.Wrap(
			apperrors.CodeUnavailable,
			fmt.Sprintf("mcp request failed for method %s at %s", req.Method, safeEndpoint(endpoint)),
			err,
		)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 2048))
		return rpcResponse{}, apperrors.New(
			apperrors.CodeUnavailable,
			fmt.Sprintf("mcp server returned status %d for method %s at %s: %s", httpResp.StatusCode, req.Method, safeEndpoint(endpoint), strings.TrimSpace(string(body))),
		)
	}
	sessionID = firstNonEmpty(httpResp.Header.Get("Mcp-Session-Id"), httpResp.Header.Get("MCP-Session-Id"))
	if !expectResponse || httpResp.StatusCode == http.StatusAccepted {
		return rpcResponse{SessionID: sessionID}, nil
	}
	contentType := httpResp.Header.Get("Content-Type")
	raw, err := readRPCResponseBody(httpResp.Body, contentType)
	if err != nil {
		return rpcResponse{}, err
	}
	msg, err := decodeRPCResponse(raw, contentType)
	if err != nil {
		return rpcResponse{}, err
	}
	msg.SessionID = sessionID
	if msg.Error != nil {
		return rpcResponse{}, apperrors.New(apperrors.CodeUnavailable, fmt.Sprintf("mcp json-rpc error %d: %s", msg.Error.Code, msg.Error.Message))
	}
	return msg, nil
}

func normalizeEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "mcp server url must be a valid absolute http(s) url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "mcp server url must use http or https")
	}
	return raw, nil
}

func safeEndpoint(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return raw
	}
	return parsed.Scheme + "://" + parsed.Host + parsed.Path
}

func readRPCResponseBody(body io.Reader, contentType string) ([]byte, error) {
	if isSSEContent(contentType) {
		payload, err := firstSSEDataPayloadFromReader(io.LimitReader(body, 8<<20))
		if err != nil {
			return nil, err
		}
		return []byte(payload), nil
	}
	raw, err := io.ReadAll(io.LimitReader(body, 8<<20))
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to read mcp response", err)
	}
	return raw, nil
}

func decodeRPCResponse(raw []byte, contentType string) (rpcResponse, error) {
	trimmed := bytes.TrimSpace(raw)
	if (isSSEContent(contentType) || bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte("data:"))) && !bytes.HasPrefix(trimmed, []byte("{")) {
		payload, err := firstSSEDataPayload(string(raw))
		if err != nil {
			return rpcResponse{}, err
		}
		trimmed = []byte(payload)
	}
	var msg rpcResponse
	if err := json.Unmarshal(trimmed, &msg); err != nil {
		return rpcResponse{}, apperrors.Wrap(apperrors.CodeInternal, "failed to decode mcp json-rpc response", err)
	}
	return msg, nil
}

func firstSSEDataPayloadFromReader(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 8<<20)

	var dataLines []string
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if payload, ok := jsonPayloadFromSSEData(dataLines); ok {
				return payload, nil
			}
			dataLines = nil
			continue
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if payload, ok := jsonPayloadFromSSEData(dataLines); ok {
		return payload, nil
	}
	if err := scanner.Err(); err != nil {
		return "", apperrors.Wrap(apperrors.CodeInternal, "failed to read mcp sse response", err)
	}
	return "", apperrors.New(apperrors.CodeInternal, "mcp sse response did not contain a json data payload")
}

func firstSSEDataPayload(raw string) (string, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	events := strings.Split(raw, "\n\n")
	for _, event := range events {
		var dataLines []string
		for _, line := range strings.Split(event, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if len(dataLines) == 0 {
			continue
		}
		if payload, ok := jsonPayloadFromSSEData(dataLines); ok {
			return payload, nil
		}
	}
	return "", apperrors.New(apperrors.CodeInternal, "mcp sse response did not contain a json data payload")
}

func jsonPayloadFromSSEData(dataLines []string) (string, bool) {
	if len(dataLines) == 0 {
		return "", false
	}
	data := strings.Join(dataLines, "\n")
	if strings.HasPrefix(strings.TrimSpace(data), "{") {
		return data, true
	}
	return "", false
}

func isSSEContent(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC   string          `json:"jsonrpc"`
	ID        any             `json:"id,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *rpcError       `json:"error,omitempty"`
	SessionID string          `json:"-"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolsListResult struct {
	Tools      []remoteTool `json:"tools"`
	NextCursor string       `json:"nextCursor"`
}

type remoteTool struct {
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	InputSchema       map[string]any `json:"inputSchema"`
	InputSchemaSnake  map[string]any `json:"input_schema"`
	OutputSchema      map[string]any `json:"outputSchema"`
	OutputSchemaSnake map[string]any `json:"output_schema"`
}

func (t remoteTool) toTool() Tool {
	inputSchema := t.InputSchema
	if inputSchema == nil {
		inputSchema = t.InputSchemaSnake
	}
	outputSchema := t.OutputSchema
	if outputSchema == nil {
		outputSchema = t.OutputSchemaSnake
	}
	return Tool{
		Name:         t.Name,
		Description:  t.Description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}
}

type toolCallResult struct {
	Content           []map[string]any `json:"content"`
	StructuredContent map[string]any   `json:"structuredContent"`
	IsError           bool             `json:"isError"`
}

func firstTextContent(content []map[string]any) string {
	for _, item := range content {
		if item["type"] == "text" {
			if text, ok := item["text"].(string); ok {
				return text
			}
		}
	}
	return "mcp tool returned an error"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
