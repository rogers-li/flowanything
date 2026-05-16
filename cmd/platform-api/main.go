package main

import (
	"context"
	"os"

	"flow-anything/internal/platform/kernel/config"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/logging"
	"flow-anything/internal/platformapi/application"
	"flow-anything/internal/platformapi/infrastructure"
	httpapi "flow-anything/internal/platformapi/interfaces/http"
)

func main() {
	logger := logging.New("platform-api")
	registry, err := infrastructure.OpenSQLiteRegistry(context.Background(), config.String("PLATFORM_DB_DSN", "file:./flow-anything.db?cache=shared"))
	if err != nil {
		logger.Error("failed to initialize registry", "error", err)
		os.Exit(1)
	}
	defer registry.Close()

	app := application.New(logger, registry, registry, registry, registry, registry, registry, registry)

	server := httpserver.New(
		"platform-api",
		httpserver.AddrFromEnv("PLATFORM_API_ADDR", ":8080"),
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
