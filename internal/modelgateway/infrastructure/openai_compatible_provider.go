package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"flow-anything/internal/modelgateway/ports"
	"flow-anything/internal/platform/contracts/model"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type OpenAICompatibleConfig struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	Organization string
	Project      string
	Timeout      time.Duration
	ExtraBody    map[string]any
}

type OpenAICompatibleProvider struct {
	config OpenAICompatibleConfig
	client *http.Client
}

func NewOpenAICompatibleProvider(config OpenAICompatibleConfig) (*OpenAICompatibleProvider, error) {
	return NewOpenAICompatibleProviderWithClient(config, nil)
}

func NewOpenAICompatibleProviderWithClient(config OpenAICompatibleConfig, client *http.Client) (*OpenAICompatibleProvider, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.DefaultModel = strings.TrimSpace(config.DefaultModel)
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.BaseURL == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "openai compatible base url is required")
	}
	if config.APIKey == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "openai compatible api key is required")
	}
	if config.DefaultModel == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "openai compatible model is required")
	}
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}

	return &OpenAICompatibleProvider{
		config: config,
		client: client,
	}, nil
}

func (p *OpenAICompatibleProvider) ChatProviderMetadata() ports.ChatProviderMetadata {
	return ports.ChatProviderMetadata{
		Name:         "openai-compatible",
		BaseURL:      p.config.BaseURL,
		DefaultModel: p.config.DefaultModel,
	}
}

// Chat translates the platform model contract into the OpenAI-compatible chat
// completions protocol, including tool definitions and tool call results.
func (p *OpenAICompatibleProvider) Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error) {
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = p.config.DefaultModel
	}
	if modelName == "" {
		return model.ChatResponse{}, apperrors.New(apperrors.CodeInvalidArgument, "model is required")
	}

	payload := openAIChatRequest{
		Model:       modelName,
		Messages:    toOpenAIMessages(req.Messages),
		Tools:       toOpenAITools(req.Tools),
		ToolChoice:  req.ToolChoice,
		Temperature: req.Options.Temperature,
		MaxTokens:   req.Options.MaxTokens,
	}
	body, err := marshalOpenAIChatRequest(payload, p.config.ExtraBody)
	if err != nil {
		return model.ChatResponse{}, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to encode model request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return model.ChatResponse{}, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to build model request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	if p.config.Organization != "" {
		httpReq.Header.Set("OpenAI-Organization", p.config.Organization)
	}
	if p.config.Project != "" {
		httpReq.Header.Set("OpenAI-Project", p.config.Project)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return model.ChatResponse{}, apperrors.Wrap(apperrors.CodeUnavailable, "model request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return model.ChatResponse{}, decodeOpenAIError(resp)
	}

	var decoded openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return model.ChatResponse{}, apperrors.Wrap(apperrors.CodeInternal, "failed to decode model response", err)
	}
	if len(decoded.Choices) == 0 {
		return model.ChatResponse{}, apperrors.New(apperrors.CodeInternal, "model response has no choices")
	}

	responseID := id.ID(decoded.ID)
	if responseID.Empty() {
		responseID = id.New("chatcmpl")
	}
	createdAt := time.Now().UTC()
	if decoded.Created > 0 {
		createdAt = time.Unix(decoded.Created, 0).UTC()
	}
	choice := decoded.Choices[0]

	return model.ChatResponse{
		ID:        responseID,
		RequestID: req.ID,
		TraceID:   req.TraceID,
		Model:     decoded.Model,
		Message: model.Message{
			Role:      model.Role(choice.Message.Role),
			Content:   choice.Message.Content,
			ToolCalls: fromOpenAIToolCalls(choice.Message.ToolCalls),
		},
		FinishReason: choice.FinishReason,
		Usage: model.Usage{
			InputTokens:  decoded.Usage.PromptTokens,
			OutputTokens: decoded.Usage.CompletionTokens,
			TotalTokens:  decoded.Usage.TotalTokens,
		},
		CreatedAt: createdAt,
	}, nil
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  string          `json:"tool_choice,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

func marshalOpenAIChatRequest(payload openAIChatRequest, extraBody map[string]any) ([]byte, error) {
	if len(extraBody) == 0 {
		return json.Marshal(payload)
	}

	base, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}
	for key, value := range extraBody {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		merged[key] = value
	}

	return json.Marshal(merged)
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolCallFunction `json:"function"`
}

type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Created int64  `json:"created"`
	Choices []struct {
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func toOpenAIMessages(messages []model.Message) []openAIMessage {
	result := make([]openAIMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, openAIMessage{
			Role:       string(message.Role),
			Content:    message.Content,
			ToolCalls:  toOpenAIToolCalls(message.ToolCalls),
			ToolCallID: message.ToolCallID,
		})
	}
	return result
}

func toOpenAITools(tools []model.ToolDefinition) []openAITool {
	result := make([]openAITool, 0, len(tools))
	for _, toolDef := range tools {
		toolType := toolDef.Type
		if toolType == "" {
			toolType = "function"
		}
		result = append(result, openAITool{
			Type: toolType,
			Function: openAIToolFunction{
				Name:        toolDef.Function.Name,
				Description: toolDef.Function.Description,
				Parameters:  toolDef.Function.Parameters,
			},
		})
	}
	return result
}

func toOpenAIToolCalls(toolCalls []model.ToolCall) []openAIToolCall {
	result := make([]openAIToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		args, _ := json.Marshal(call.Function.Arguments)
		callType := call.Type
		if callType == "" {
			callType = "function"
		}
		result = append(result, openAIToolCall{
			ID:   call.ID,
			Type: callType,
			Function: openAIToolCallFunction{
				Name:      call.Function.Name,
				Arguments: string(args),
			},
		})
	}
	return result
}

func fromOpenAIToolCalls(toolCalls []openAIToolCall) []model.ToolCall {
	result := make([]model.ToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		args := map[string]any{}
		if call.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
		}
		result = append(result, model.ToolCall{
			ID:   call.ID,
			Type: call.Type,
			Function: model.ToolCallFunction{
				Name:      call.Function.Name,
				Arguments: args,
			},
		})
	}
	return result
}

func decodeOpenAIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var decoded struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && decoded.Error.Message != "" {
		return apperrors.New(codeFromStatus(resp.StatusCode), decoded.Error.Message)
	}

	return apperrors.New(codeFromStatus(resp.StatusCode), fmt.Sprintf("model request failed with status %d", resp.StatusCode))
}

func codeFromStatus(statusCode int) apperrors.Code {
	switch statusCode {
	case http.StatusBadRequest:
		return apperrors.CodeInvalidArgument
	case http.StatusUnauthorized:
		return apperrors.CodeUnauthorized
	case http.StatusForbidden:
		return apperrors.CodeForbidden
	case http.StatusNotFound:
		return apperrors.CodeNotFound
	case http.StatusConflict:
		return apperrors.CodeConflict
	case http.StatusServiceUnavailable:
		return apperrors.CodeUnavailable
	default:
		return apperrors.CodeInternal
	}
}
