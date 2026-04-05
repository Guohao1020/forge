# Forge Platform — Build & Development Commands
#
# Usage:
#   make dev        Start local dev environment
#   make test       Run all tests
#   make build      Build all services
#   make docker     Build all Docker images
#   make deploy     Deploy full stack via docker-compose

.PHONY: dev dev-stop test build docker deploy clean

# === Development ===

dev:
	bash scripts/dev-start.sh

dev-stop:
	bash scripts/dev-stop.sh

# === Testing ===

test: test-go test-python test-ts test-lint

test-go:
	cd forge-core && go test ./internal/... -count=1
	cd forge-bot && go test ./... -count=1

test-python:
	cd ai-worker && python -m pytest tests/ --tb=short -q

test-ts:
	cd forge-portal && npx tsc --noEmit --pretty false

test-lint:
	cd forge-portal && npx eslint app/ components/ lib/ --max-warnings 50

test-api:
	bash scripts/test-api.sh

smoke-test:
	bash scripts/smoke-test.sh

bench:
	cd forge-core && go test ./internal/... -bench=. -benchmem -count=1 -run=^$$

# === Build ===

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build: build-core build-bot build-portal

build-core:
	cd forge-core && go build -ldflags "-X github.com/shulex/forge/forge-core/internal/middleware.Version=$(VERSION)" -o forge-core ./cmd/forge-core

build-bot:
	cd forge-bot && go build -o forge-bot ./cmd/forge-bot

build-portal:
	cd forge-portal && npm run build

# === Docker ===

docker:
	docker build --build-arg VERSION=$(VERSION) -t forge-core:$(VERSION) -t forge-core:latest ./forge-core
	docker build -t forge-ai-worker:latest ./ai-worker
	docker build -t forge-portal:latest ./forge-portal
	docker build -t forge-bot:latest ./forge-bot
	docker build -t forge-task-runner:latest ./docker/task-runner

# === Deploy ===

deploy:
	docker compose -f docker-compose.prod.yml up -d --build

deploy-down:
	docker compose -f docker-compose.prod.yml down

# === Infrastructure Only ===

infra:
	docker compose -f docker-compose.dev.yml up -d postgres redis temporal

infra-down:
	docker compose -f docker-compose.dev.yml down

# === Clean ===

clean:
	rm -f forge-core/forge-core forge-core/forge-core.exe
	rm -rf forge-portal/.next forge-portal/out
	find ai-worker -name __pycache__ -type d -exec rm -rf {} + 2>/dev/null || true
