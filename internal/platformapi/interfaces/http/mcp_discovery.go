package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/mcpclient"
)

type mcpDiscoveryRequest struct {
	Name                 string            `json:"name"`
	URL                  string            `json:"url"`
	Transport            string            `json:"transport"`
	Headers              []mcpHeader       `json:"headers"`
	HeaderMap            map[string]string `json:"header_map"`
	RequireAuthorization bool              `json:"require_authorization"`
}

type mcpHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type mcpDiscoveryResponse struct {
	ServerID string              `json:"server_id"`
	Tools    []mcpDiscoveredTool `json:"tools"`
}

type mcpDiscoveredTool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

func registerMCPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/mcp/servers/discover", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req mcpDiscoveryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid mcp discovery json")
			return
		}

		client := mcpclient.New(30 * time.Second)
		tools, err := client.DiscoverTools(r.Context(), mcpclient.ServerConfig{
			Name:      strings.TrimSpace(req.Name),
			URL:       strings.TrimSpace(req.URL),
			Transport: strings.TrimSpace(req.Transport),
			Headers:   mcpHeadersToMap(req),
		})
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		resp := mcpDiscoveryResponse{
			ServerID: strings.TrimSpace(req.Name),
			Tools:    make([]mcpDiscoveredTool, 0, len(tools)),
		}
		for _, item := range tools {
			resp.Tools = append(resp.Tools, mcpDiscoveredTool{
				Name:         item.Name,
				Description:  item.Description,
				InputSchema:  item.InputSchema,
				OutputSchema: item.OutputSchema,
			})
		}
		httpserver.WriteJSON(w, http.StatusOK, resp)
	})
}

func mcpHeadersToMap(req mcpDiscoveryRequest) map[string]string {
	result := make(map[string]string, len(req.Headers)+len(req.HeaderMap))
	for name, value := range req.HeaderMap {
		if strings.TrimSpace(name) != "" {
			result[strings.TrimSpace(name)] = value
		}
	}
	for _, header := range req.Headers {
		if strings.TrimSpace(header.Name) != "" {
			result[strings.TrimSpace(header.Name)] = header.Value
		}
	}
	return result
}
