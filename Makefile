# Airgapper Makefile

BINARY := airgapper
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
GO := go

.PHONY: all build install test clean lint docker docker-push help

all: build

## Build

build: ## Build the binary
	$(GO) build $(LDFLAGS) -o bin/$(BINARY) ./cmd/airgapper

install: ## Install to $GOPATH/bin
	$(GO) install $(LDFLAGS) ./cmd/airgapper

## Development

test: ## Run tests
	$(GO) test -v ./...

test-cover: ## Run tests with coverage
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

lint: ## Run linters
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

fmt: ## Format code
	$(GO) fmt ./...

## Docker

docker: ## Build Docker image
	docker build -t airgapper:latest -f docker/Dockerfile .

docker-compose-up: ## Start docker-compose stack
	docker-compose -f docker/docker-compose.yml up -d

docker-compose-down: ## Stop docker-compose stack
	docker-compose -f docker/docker-compose.yml down

docker-compose-logs: ## View docker-compose logs
	docker-compose -f docker/docker-compose.yml logs -f

## Demo / Testing

demo-init: ## Initialize demo environment
	@echo "Starting restic-rest-server..."
	docker run -d --name restic-rest-server -p 8000:8000 \
		-v restic-data:/data restic/rest-server --append-only --no-auth || true
	@echo ""
	@echo "REST server running at http://localhost:8000"
	@echo ""
	@echo "Initialize Airgapper:"
	@echo "  airgapper init --name alice --repo rest:http://localhost:8000/mybackup"

demo-stop: ## Stop demo environment
	docker stop restic-rest-server || true
	docker rm restic-rest-server || true

## Cleanup

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

clean-all: clean ## Clean everything including Docker volumes
	docker-compose -f docker/docker-compose.yml down -v || true
	docker volume rm restic-data || true

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
