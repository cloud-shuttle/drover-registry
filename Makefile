# drover-registry Makefile
# Registry service for versioned agent/crew packages

.PHONY: all build run test clean deps help lint docker docker-build docker-up docker-down docker-logs migrate-up migrate-down dev-stack

# Variables
BINARY_NAME=drover-registry
BUILD_DIR=./build
GO?=go
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS?=-ldflags "-X main.version=$(VERSION)"
PORT?=8080

all: build

deps:
	$(GO) mod download
	$(GO) mod tidy

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server
	@echo "✅ Built $(BUILD_DIR)/$(BINARY_NAME)"

run: build
	@echo "Running $(BINARY_NAME) on :$(PORT)..."
	PORT=$(PORT) $(BUILD_DIR)/$(BINARY_NAME)

dev:
	@echo "Running in dev mode (go run)..."
	PORT=$(PORT) $(GO) run ./cmd/server

test:
	$(GO) test -v ./...

test-race:
	$(GO) test -race -v ./...

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean -cache
	@echo "Clean!"

lint:
	@echo "Running go vet..."
	$(GO) vet ./...
	@echo "✅ vet passed (add golangci-lint for full lint)"

# === Dev Stack (Docker Compose) ===

docker-build:
	docker compose build registry

docker-up:
	docker compose up -d postgres minio registry
	@echo "✅ Registry stack is starting. API: http://localhost:8080"
	@echo "   MinIO console: http://localhost:9001 (minioadmin / minioadmin)"

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f registry

# Run migrations using the official migrate image (requires migrations/ dir)
migrate-up:
	docker compose --profile migrate run --rm migrate

migrate-down:
	docker compose --profile migrate run --rm migrate down 1

# One-command friendly dev experience
dev-stack: docker-up migrate-up
	@echo ""
	@echo "🚀 drover-registry dev stack ready!"
	@echo "   Health:  curl http://localhost:8080/healthz"
	@echo "   Publish: curl -X POST 'http://localhost:8080/v1/packages?name=my-crew&version=v1.0.0' --data-binary @my-crew.tar.gz -H 'X-Org-ID: my-org'"
	@echo "   Use X-Org-ID header for tenant in dev mode."

docker-build:
	docker build -t ghcr.io/cloud-shuttle/drover-registry:$(VERSION) -f deploy/Dockerfile .

docker-run: docker-build
	docker run --rm -p $(PORT):8080 ghcr.io/cloud-shuttle/drover-registry:$(VERSION)

help:
	@echo "drover-registry - Distributed Agent Registry & Crew Store"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build the server binary"
	@echo "  run          Build + run the server (PORT=8080)"
	@echo "  dev          Run with 'go run' (hot reload friendly)"
	@echo "  test         Run all tests"
	@echo "  test-race    Run tests with race detector"
	@echo "  deps         Tidy modules"
	@echo "  clean        Remove build artifacts"
	@echo "  lint         Static checks"
	@echo "  help         This help"
