package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"flow-anything/internal_new/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config := bootstrap.RuntimeConfigFromEnv("FLOW_ANYTHING")
	if err := bootstrap.RunHTTP(ctx, config); err != nil {
		log.Printf("ai-platform-runtime stopped with error: %v", err)
		os.Exit(1)
	}
}
