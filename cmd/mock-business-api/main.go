package main

import (
	"os"

	"flow-anything/internal/mockbusiness"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/logging"
)

func main() {
	logger := logging.New("mock-business-api")
	server := httpserver.New(
		"mock-business-api",
		httpserver.AddrFromEnv("MOCK_BUSINESS_API_ADDR", ":8090"),
		logger,
	)

	mockbusiness.RegisterRoutes(server.Mux())

	ctx, stop := httpserver.SignalContext()
	defer stop()

	if err := server.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
