package main

import (
	"os"

	"flow-anything/internal/connector/application"
	"flow-anything/internal/connector/infrastructure"
	httpapi "flow-anything/internal/connector/interfaces/http"
	"flow-anything/internal/platform/kernel/config"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/logging"
)

func main() {
	logger := logging.New("connector-service")
	operations := infrastructure.NewHTTPOperationRepository(config.String("PLATFORM_API_URL", "http://localhost:8080"))
	invoker := infrastructure.NewHTTPOperationInvoker()
	app := application.New(logger, operations, invoker)

	server := httpserver.New(
		"connector-service",
		httpserver.AddrFromEnv("CONNECTOR_SERVICE_ADDR", ":8083"),
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
