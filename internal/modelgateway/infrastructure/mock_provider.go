package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"flow-anything/internal/modelgateway/ports"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/kernel/id"
)

const DefaultMockModelName = "mock-chat"

type MockProviderConfig struct {
	ModelName string
}

type MockProviderOption func(*MockProviderConfig)

type MockProvider struct {
	config MockProviderConfig
}

func NewMockProvider(opts ...MockProviderOption) *MockProvider {
	config := MockProviderConfig{ModelName: DefaultMockModelName}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	if strings.TrimSpace(config.ModelName) == "" {
		config.ModelName = DefaultMockModelName
	}
	return &MockProvider{config: config}
}

// WithMockModelName configures the model name returned by the local mock
// provider. Keeping it configurable helps tests and local environments mirror
// production routing metadata without changing provider behavior.
func WithMockModelName(modelName string) MockProviderOption {
	return func(config *MockProviderConfig) {
		if strings.TrimSpace(modelName) != "" {
			config.ModelName = strings.TrimSpace(modelName)
		}
	}
}

func (p *MockProvider) ChatProviderMetadata() ports.ChatProviderMetadata {
	return ports.ChatProviderMetadata{
		Name:         "mock",
		BaseURL:      "local",
		DefaultModel: p.config.ModelName,
	}
}

// Chat implements a deterministic local model provider for development and
// integration tests. When tools are supplied, it emits one synthetic tool call;
// after a tool result is present, it returns a final assistant message.
func (p *MockProvider) Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error) {
	modelName := req.Model
	if modelName == "" {
		modelName = p.config.ModelName
	}
	if len(req.Tools) > 0 && !lastMessageIsToolResult(req.Messages) {
		return mockToolCallResponse(req, modelName), nil
	}
	if lastMessageIsToolResult(req.Messages) {
		return mockFinalResponse(req, modelName), nil
	}

	userText := lastUserMessage(req.Messages)
	content := "这是一个本地 mock 模型回复。"
	if userText != "" {
		content = fmt.Sprintf("这是一个本地 mock 模型回复：我理解你说的是「%s」。", userText)
	}

	inputTokens := estimateTokens(messagesText(req.Messages))
	outputTokens := estimateTokens(content)

	return model.ChatResponse{
		ID:        id.New("chatcmpl"),
		RequestID: req.ID,
		TraceID:   req.TraceID,
		Model:     modelName,
		Message: model.Message{
			Role:    model.RoleAssistant,
			Content: content,
		},
		FinishReason: "stop",
		Usage: model.Usage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
		CreatedAt: time.Now().UTC(),
	}, nil
}

func mockToolCallResponse(req model.ChatRequest, modelName string) model.ChatResponse {
	toolDef := req.Tools[0]
	userText := lastUserMessage(req.Messages)
	args := map[string]any{
		"query": userText,
	}
	if orderID := extractOrderID(userText); orderID != "" {
		args["order_id"] = orderID
	}
	if shouldProvideWeatherCity(toolDef) {
		args["city"] = extractWeatherCity(userText)
	}

	return model.ChatResponse{
		ID:        id.New("chatcmpl"),
		RequestID: req.ID,
		TraceID:   req.TraceID,
		Model:     modelName,
		Message: model.Message{
			Role: model.RoleAssistant,
			ToolCalls: []model.ToolCall{
				{
					ID:   id.New("toolcall").String(),
					Type: "function",
					Function: model.ToolCallFunction{
						Name:      toolDef.Function.Name,
						Arguments: args,
					},
				},
			},
		},
		FinishReason: "tool_calls",
		CreatedAt:    time.Now().UTC(),
	}
}

func mockFinalResponse(req model.ChatRequest, modelName string) model.ChatResponse {
	toolText := lastToolMessage(req.Messages)
	content := "工具执行完成，我已经拿到结果。"
	if toolText != "" {
		content = "工具执行完成，结果摘要：" + toolText
	}

	return model.ChatResponse{
		ID:        id.New("chatcmpl"),
		RequestID: req.ID,
		TraceID:   req.TraceID,
		Model:     modelName,
		Message: model.Message{
			Role:    model.RoleAssistant,
			Content: content,
		},
		FinishReason: "stop",
		Usage: model.Usage{
			InputTokens:  estimateTokens(messagesText(req.Messages)),
			OutputTokens: estimateTokens(content),
			TotalTokens:  estimateTokens(messagesText(req.Messages)) + estimateTokens(content),
		},
		CreatedAt: time.Now().UTC(),
	}
}

func lastUserMessage(messages []model.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == model.RoleUser {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func lastToolMessage(messages []model.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == model.RoleTool {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func lastMessageIsToolResult(messages []model.Message) bool {
	if len(messages) == 0 {
		return false
	}
	return messages[len(messages)-1].Role == model.RoleTool
}

func extractOrderID(text string) string {
	re := regexp.MustCompile(`(?i)\b[o][-_]?[a-z0-9]+\b`)
	match := re.FindString(text)
	if match == "" {
		return ""
	}
	return match
}

func shouldProvideWeatherCity(toolDef model.ToolDefinition) bool {
	if strings.Contains(strings.ToLower(toolDef.Function.Name), "weather") {
		return true
	}

	properties, _ := toolDef.Function.Parameters["properties"].(map[string]any)
	_, hasCity := properties["city"]
	return hasCity
}

func extractWeatherCity(text string) string {
	knownCities := []string{
		"深圳", "北京", "上海", "广州", "杭州",
	}
	for _, city := range knownCities {
		if strings.Contains(text, city) {
			return city
		}
	}

	lower := strings.ToLower(text)
	englishCities := []string{
		"shenzhen", "beijing", "shanghai", "guangzhou", "hangzhou",
	}
	for _, city := range englishCities {
		if strings.Contains(lower, city) {
			return city
		}
	}

	return "深圳"
}

func messagesText(messages []model.Message) string {
	var builder strings.Builder
	for _, message := range messages {
		builder.WriteString(message.Content)
		if len(message.ToolCalls) > 0 {
			bytes, _ := json.Marshal(message.ToolCalls)
			builder.Write(bytes)
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return len([]rune(text))/4 + 1
}
