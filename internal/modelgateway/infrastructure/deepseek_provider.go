package infrastructure

import (
	"context"
	"strings"
	"time"

	"flow-anything/internal/modelgateway/ports"
	"flow-anything/internal/platform/contracts/model"
)

const (
	DefaultDeepSeekBaseURL = "https://api.deepseek.com"
	DefaultDeepSeekModel   = "deepseek-v4-flash"
)

type DeepSeekConfig struct {
	BaseURL         string
	APIKey          string
	DefaultModel    string
	ThinkingType    string
	ReasoningEffort string
	Timeout         time.Duration
}

type DeepSeekProvider struct {
	delegate        *OpenAICompatibleProvider
	baseURL         string
	defaultModel    string
	thinkingType    string
	reasoningEffort string
}

func NewDeepSeekProvider(config DeepSeekConfig) (*DeepSeekProvider, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = DefaultDeepSeekBaseURL
	}
	if strings.TrimSpace(config.DefaultModel) == "" {
		config.DefaultModel = DefaultDeepSeekModel
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Minute
	}

	delegate, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:      config.BaseURL,
		APIKey:       config.APIKey,
		DefaultModel: config.DefaultModel,
		Timeout:      config.Timeout,
		ExtraBody:    deepSeekExtraBody(config),
	})
	if err != nil {
		return nil, err
	}

	return &DeepSeekProvider{
		delegate:        delegate,
		baseURL:         delegate.config.BaseURL,
		defaultModel:    delegate.config.DefaultModel,
		thinkingType:    strings.TrimSpace(config.ThinkingType),
		reasoningEffort: strings.TrimSpace(config.ReasoningEffort),
	}, nil
}

func (p *DeepSeekProvider) Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error) {
	return p.delegate.Chat(ctx, req)
}

func (p *DeepSeekProvider) ChatProviderMetadata() ports.ChatProviderMetadata {
	return ports.ChatProviderMetadata{
		Name:         "deepseek",
		BaseURL:      p.baseURL,
		DefaultModel: p.defaultModel,
		Attributes: map[string]string{
			"thinking":         p.thinkingType,
			"reasoning_effort": p.reasoningEffort,
		},
	}
}

func deepSeekExtraBody(config DeepSeekConfig) map[string]any {
	extra := map[string]any{}
	if thinkingType := strings.TrimSpace(config.ThinkingType); thinkingType != "" {
		extra["thinking"] = map[string]any{
			"type": thinkingType,
		}
	}
	if reasoningEffort := strings.TrimSpace(config.ReasoningEffort); reasoningEffort != "" {
		extra["reasoning_effort"] = reasoningEffort
	}
	return extra
}
