# drover-registry Makefile
# Registry service for versioned agent/crew packages

.PHONY: all build run test clean deps help lint docker

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
