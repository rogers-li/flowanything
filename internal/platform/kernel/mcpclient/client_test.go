package mcpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClientDiscoverToolsFromStreamableHTTPServer(t *testing.T) {
	var sawCustomHeader bool
	client := &Client{httpClient: &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Test-Token") == "secret" {
			sawCustomHeader = true
		}
		if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			t.Fatalf("expected MCP accept header to include text/event-stream, got %q", r.Header.Get("Accept"))
		}

		req := decodeTestRPCRequest(t, r)
		switch req.Method {
		case "initialize":
			return testRPCResultResponse(t, http.StatusOK, map[string]string{"Mcp-Session-Id": "session-1"}, req.ID, map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]any{},
			}), nil
		case "notifications/initialized":
			if r.Header.Get("Mcp-Session-Id") != "session-1" {
				t.Fatalf("expected initialized notification to carry session id")
			}
			return testResponse(http.StatusAccepted, nil, ""), nil
		case "tools/list":
			if r.Header.Get("Mcp-Session-Id") != "session-1" {
				t.Fatalf("expected tools/list to carry session id")
			}
			return testRPCResultResponse(t, http.StatusOK, nil, req.ID, map[string]any{
				"tools": []map[string]any{
					{
						"name":        "query_weather",
						"description": "Query weather by city.",
						"inputSchema": map[string]any{
							"type":     "object",
							"required": []string{"city"},
							"properties": map[string]any{
								"city": map[string]any{
									"type":        "string",
									"description": "City name.",
								},
							},
						},
					},
				},
			}), nil
		default:
			t.Fatalf("unexpected MCP method %s", req.Method)
		}
		return nil, nil
	})}}

	tools, err := client.DiscoverTools(context.Background(), ServerConfig{
		Name:    "test",
		URL:     "https://mcp.example.test",
		Headers: map[string]string{"X-Test-Token": "secret"},
	})
	if err != nil {
		t.Fatalf("DiscoverTools returned error: %v", err)
	}
	if !sawCustomHeader {
		t.Fatalf("expected custom headers to be sent to the MCP server")
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "query_weather" {
		t.Fatalf("unexpected tool name %q", tools[0].Name)
	}
	if tools[0].InputSchema["type"] != "object" {
		t.Fatalf("expected input schema to be preserved")
	}
}

func TestClientCallToolFromSSEResponse(t *testing.T) {
	client := &Client{httpClient: &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		req := decodeTestRPCRequest(t, r)
		switch req.Method {
		case "initialize":
			return testRPCResultResponse(t, http.StatusOK, map[string]string{"Mcp-Session-Id": "session-2"}, req.ID, map[string]any{"protocolVersion": protocolVersion}), nil
		case "notifications/initialized":
			return testResponse(http.StatusAccepted, nil, ""), nil
		case "tools/call":
			return testResponse(
				http.StatusOK,
				map[string]string{"Content-Type": "text/event-stream"},
				"event: message\r\ndata: {\"jsonrpc\":\"2.0\",\"id\":3,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Sunny\"}],\"structuredContent\":{\"temperature\":26}}}\r\n\r\n",
			), nil
		default:
			t.Fatalf("unexpected MCP method %s", req.Method)
		}
		return nil, nil
	})}}

	result, err := client.CallTool(context.Background(), ServerConfig{
		Name: "test",
		URL:  "https://mcp.example.test",
	}, "query_weather", map[string]any{"city": "Singapore"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected call to succeed, got error %s: %s", result.ErrorCode, result.ErrorReason)
	}
	if result.Data["structured_content"].(map[string]any)["temperature"].(float64) != 26 {
		t.Fatalf("expected structured content from MCP response")
	}
}

func decodeTestRPCRequest(t *testing.T, r *http.Request) rpcRequest {
	t.Helper()
	defer r.Body.Close()
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Fatalf("failed to decode rpc request: %v", err)
	}
	return req
}

func writeTestRPCResult(t *testing.T, w http.ResponseWriter, id any, result map[string]any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}); err != nil {
		t.Fatalf("failed to write rpc response: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func testRPCResultResponse(t *testing.T, status int, headers map[string]string, id any, result map[string]any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	if err != nil {
		t.Fatalf("failed to encode rpc response: %v", err)
	}
	return testResponse(status, headers, string(payload))
}

func testResponse(status int, headers map[string]string, body string) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	for name, value := range headers {
		resp.Header.Set(name, value)
	}
	return resp
}
