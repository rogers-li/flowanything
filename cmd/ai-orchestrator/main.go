package main

import (
	"os"
	"time"

	"flow-anything/internal/aiorchestrator/application"
	"flow-anything/internal/aiorchestrator/infrastructure"
	httpapi "flow-anything/internal/aiorchestrator/interfaces/http"
	"flow-anything/internal/contextengine"
	"flow-anything/internal/platform/kernel/config"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/logging"
	"flow-anything/internal/platform/kernel/runtimeevents"
)

func main() {
	logger := logging.New("ai-orchestrator")
	toolRuntime := infrastructure.NewHTTPToolRuntime(
		config.String("AGENT_RUNTIME_URL", "http://localhost:8082"),
		config.Duration("AI_ORCHESTRATOR_TOOL_RUNTIME_TIMEOUT", 75*time.Second),
	)
	modelClient := infrastructure.NewHTTPModelClientWithTimeout(
		config.String("MODEL_GATEWAY_URL", "http://localhost:8085"),
		config.Duration("AI_ORCHESTRATOR_MODEL_GATEWAY_TIMEOUT", 10*time.Minute),
	)
	configLoader := infrastructure.NewHTTPAgentConfigLoader(config.String("PLATFORM_API_URL", "http://localhost:8080"))
	runtimeEvents := runtimeevents.NewBroker()
	app := application.New(
		logger,
		toolRuntime,
		modelClient,
		configLoader,
		application.WithDefaultSystemPrompt(config.String("AI_ORCHESTRATOR_DEFAULT_SYSTEM_PROMPT", application.DefaultSystemPrompt)),
		application.WithMaxToolIterations(config.Int("AI_ORCHESTRATOR_MAX_TOOL_ITERATIONS", application.DefaultMaxToolIterations)),
		application.WithMaxHistoryMessages(config.Int("AI_ORCHESTRATOR_MAX_HISTORY_MESSAGES", application.DefaultMaxHistoryMessages)),
		application.WithContextPolicy(contextengine.Policy{
			MaxHistoryMessages:  config.Int("AI_ORCHESTRATOR_MAX_HISTORY_MESSAGES", application.DefaultMaxHistoryMessages),
			MaxApproxTokens:     config.Int("AI_ORCHESTRATOR_CONTEXT_MAX_APPROX_TOKENS", 32000),
			MaxToolResultChars:  config.Int("AI_ORCHESTRATOR_CONTEXT_MAX_TOOL_RESULT_CHARS", 12000),
			MaxMessageTextChars: config.Int("AI_ORCHESTRATOR_CONTEXT_MAX_MESSAGE_TEXT_CHARS", 12000),
		}),
		application.WithConversationStore(infrastructure.NewMemoryConversationStore()),
		application.WithTraceStore(infrastructure.NewMemoryTraceStore()),
		application.WithRuntimeEventSink(runtimeEvents),
	)

	server := httpserver.New(
		"ai-orchestrator",
		httpserver.AddrFromEnv("AI_ORCHESTRATOR_ADDR", ":8081"),
		logger,
	)
	httpapi.RegisterRoutes(server.Mux(), app, runtimeEvents)

	ctx, stop := httpserver.SignalContext()
	defer stop()

	if err := server.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
