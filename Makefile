GOCACHE ?= /tmp/flow-anything-gocache

.PHONY: fmt test web-install web-dev web-build website-install website-dev website-build website-serve start-services stop-services restart-services oss-check seed-weather-flow seed-juhe-news-connector seed-feishu-doc-connector run-ai-platform-runtime

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

website-install:
	npm --prefix website install

website-dev:
	npm --prefix website run start

website-build:
	npm --prefix website run build

website-serve:
	npm --prefix website run serve

start-services:
	bash scripts/local/start-services.sh

stop-services:
	bash scripts/local/stop-services.sh

restart-services:
	bash scripts/local/restart-services.sh

oss-check:
	bash scripts/oss/preflight.sh

seed-weather-flow:
	bash scripts/local/seed-weather-flow.sh

seed-juhe-news-connector:
	bash scripts/local/seed-juhe-news-connector.sh

seed-feishu-doc-connector:
	bash scripts/local/seed-feishu-doc-connector.sh

run-ai-platform-runtime:
	GOCACHE=$(GOCACHE) go run ./cmd/ai-platform-runtime
