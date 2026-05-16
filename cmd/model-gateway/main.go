package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"flow-anything/internal/modelgateway/application"
	"flow-anything/internal/modelgateway/infrastructure"
	httpapi "flow-anything/internal/modelgateway/interfaces/http"
	"flow-anything/internal/modelgateway/ports"
	"flow-anything/internal/platform/kernel/config"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/logging"
)

func main() {
	logger := logging.New("model-gateway")
	provider, err := buildProviderFromEnv()
	if err != nil {
		logger.Error("failed to initialize model provider", "error", err)
		os.Exit(1)
	}
	app := application.New(logger, provider)

	server := httpserver.New(
		"model-gateway",
		httpserver.AddrFromEnv("MODEL_GATEWAY_ADDR", ":8085"),
		logger,
	)
	httpapi.RegisterRoutes(server.Mux(), app)

	ctx, stop := httpserver.SignalContext()
	defer stop()

	if err := server.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func buildProviderFromEnv() (ports.ChatProvider, error) {
	provider := strings.ToLower(config.String("MODEL_GATEWAY_PROVIDER", "mock"))
	switch provider {
	case "mock":
		return infrastructure.NewMockProvider(
			infrastructure.WithMockModelName(config.String("MODEL_GATEWAY_MOCK_MODEL", infrastructure.DefaultMockModelName)),
		), nil
	case "openai-compatible", "openai_compatible", "openai":
		return infrastructure.NewOpenAICompatibleProvider(infrastructure.OpenAICompatibleConfig{
			BaseURL:      config.String("OPENAI_COMPATIBLE_BASE_URL", config.String("OPENAI_BASE_URL", "https://api.openai.com/v1")),
			APIKey:       config.String("OPENAI_COMPATIBLE_API_KEY", config.String("OPENAI_API_KEY", "")),
			DefaultModel: config.String("OPENAI_COMPATIBLE_MODEL", config.String("OPENAI_MODEL", "")),
			Organization: config.String("OPENAI_COMPATIBLE_ORG", config.String("OPENAI_ORG_ID", "")),
			Project:      config.String("OPENAI_COMPATIBLE_PROJECT", config.String("OPENAI_PROJECT_ID", "")),
			Timeout:      config.Duration("OPENAI_COMPATIBLE_TIMEOUT", 10*time.Minute),
		})
	case "deepseek":
		return infrastructure.NewDeepSeekProvider(infrastructure.DeepSeekConfig{
			BaseURL:         config.String("DEEPSEEK_BASE_URL", infrastructure.DefaultDeepSeekBaseURL),
			APIKey:          config.String("DEEPSEEK_API_KEY", ""),
			DefaultModel:    config.String("DEEPSEEK_MODEL", infrastructure.DefaultDeepSeekModel),
			ThinkingType:    config.String("DEEPSEEK_THINKING", ""),
			ReasoningEffort: config.String("DEEPSEEK_REASONING_EFFORT", ""),
			Timeout:         config.Duration("DEEPSEEK_TIMEOUT", 10*time.Minute),
		})
	default:
		return nil, fmt.Errorf("unsupported MODEL_GATEWAY_PROVIDER %q", provider)
	}
}
