GOCACHE ?= /tmp/flow-anything-gocache

.PHONY: fmt test web-install web-dev web-build start-services stop-services restart-services seed-weather-flow seed-juhe-news-connector seed-feishu-doc-connector run-platform-api run-ai-orchestrator run-agent-runtime run-connector-service run-knowledge-service run-model-gateway run-mock-business-api

fmt:
	GOCACHE=$(GOCACHE) go fmt ./...

test:
	GOCACHE=$(GOCACHE) go test ./...

web-install:
	npm --prefix web/admin-console install

web-dev:
	npm --prefix web/admin-console run dev

web-build:
	npm --prefix web/admin-console run build

start-services:
	bash scripts/local/start-services.sh

stop-services:
	bash scripts/local/stop-services.sh

restart-services:
	bash scripts/local/restart-services.sh

seed-weather-flow:
	bash scripts/local/seed-weather-flow.sh

seed-juhe-news-connector:
	bash scripts/local/seed-juhe-news-connector.sh

seed-feishu-doc-connector:
	bash scripts/local/seed-feishu-doc-connector.sh

run-platform-api:
	GOCACHE=$(GOCACHE) go run ./cmd/platform-api

run-ai-orchestrator:
	GOCACHE=$(GOCACHE) go run ./cmd/ai-orchestrator

run-agent-runtime:
	GOCACHE=$(GOCACHE) go run ./cmd/agent-runtime

run-connector-service:
	GOCACHE=$(GOCACHE) go run ./cmd/connector-service

run-knowledge-service:
	GOCACHE=$(GOCACHE) go run ./cmd/knowledge-service

run-model-gateway:
	GOCACHE=$(GOCACHE) go run ./cmd/model-gateway

run-mock-business-api:
	GOCACHE=$(GOCACHE) go run ./cmd/mock-business-api
