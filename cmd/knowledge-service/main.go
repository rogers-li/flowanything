package main

import (
	"context"
	"os"

	"flow-anything/internal/knowledge/application"
	"flow-anything/internal/knowledge/infrastructure"
	httpapi "flow-anything/internal/knowledge/interfaces/http"
	"flow-anything/internal/platform/kernel/config"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/logging"
)

func main() {
	logger := logging.New("knowledge-service")
	store, err := infrastructure.OpenSQLiteStore(context.Background(), config.String("KNOWLEDGE_DB_DSN", config.String("PLATFORM_DB_DSN", "file:./flow-anything.db?cache=shared")))
	if err != nil {
		logger.Error("failed to initialize knowledge store", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	app := application.New(logger, store, store, store)

	server := httpserver.New(
		"knowledge-service",
		httpserver.AddrFromEnv("KNOWLEDGE_SERVICE_ADDR", ":8084"),
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
