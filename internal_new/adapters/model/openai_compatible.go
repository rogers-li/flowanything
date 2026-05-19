package modeladapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"flow-anything/core/agentcore"
)

// OpenAICompatibleClient adapts Chat Completions compatible providers into
// core/agentcore.ModelClient. DeepSeek can use this adapter by configuring its
// base URL and API key.
type OpenAICompatibleClient struct {
	Provider   string
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func (c OpenAICompatibleClient) Chat(ctx agentcore.Context, req agentcore.ModelRequest) (agentcore.ModelResponse, error) {
	standardCtx := contextFromAgent(ctx)
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		return agentcore.ModelResponse{}, fmt.Errorf("model base url is required")
	}
	payload := chatCompletionRequest{
		Model:       req.Model.Model,
		Messages:    toChatMessages(req.Messages),
		Temperature: req.Model.Temperature,
	}
	if req.Model.MaxTokens > 0 {
		payload.MaxTokens = req.Model.MaxTokens
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return agentcore.ModelResponse{}, err
	}
	provider := firstNonEmpty(c.Provider, req.Model.Provider, "openai-compatible")
	log.Printf(
		"model chat request provider=%s base_url=%s model=%s api_key=%s trace_id=%s",
		provider,
		baseURL,
		req.Model.Model,
		maskSecret(c.APIKey),
		req.TraceID,
	)
	httpReq, err := http.NewRequestWithContext(standardCtx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return agentcore.ModelResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return agentcore.ModelResponse{}, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return agentcore.ModelResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf(
			"model chat failed provider=%s base_url=%s model=%s status=%d trace_id=%s response=%s",
			provider,
			baseURL,
			req.Model.Model,
			resp.StatusCode,
			req.TraceID,
			string(rawBody),
		)
		return agentcore.ModelResponse{}, fmt.Errorf("model provider %q at %s returned HTTP %d: %s", provider, baseURL, resp.StatusCode, string(rawBody))
	}
	var decoded chatCompletionResponse
	if err := json.Unmarshal(rawBody, &decoded); err != nil {
		return agentcore.ModelResponse{}, err
	}
	if len(decoded.Choices) == 0 {
		return agentcore.ModelResponse{}, fmt.Errorf("model provider returned no choices")
	}
	return agentcore.ModelResponse{
		Message: agentcore.Message{
			Role:    firstNonEmpty(decoded.Choices[0].Message.Role, "assistant"),
			Content: decoded.Choices[0].Message.Content,
		},
		Raw: map[string]any{
			"id":            decoded.ID,
			"finish_reason": decoded.Choices[0].FinishReason,
			"usage":         decoded.Usage,
		},
		Usage:    decoded.Usage,
		Provider: provider,
		Model:    firstNonEmpty(decoded.Model, req.Model.Model),
		TraceID:  req.TraceID,
	}, nil
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage map[string]any `json:"usage"`
}

func toChatMessages(messages []agentcore.Message) []chatMessage {
	out := make([]chatMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, chatMessage{
			Role:    firstNonEmpty(message.Role, "user"),
			Content: message.Content,
		})
	}
	return out
}

func contextFromAgent(ctx agentcore.Context) context.Context {
	if standard, ok := ctx.(context.Context); ok {
		return standard
	}
	return context.Background()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "<empty>"
	}
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}
