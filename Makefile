# Airgapper Makefile
# Monorepo build system for frontend and backend

BINARY := airgapper
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/lcrostarosa/airgapper/backend/internal/cli.version=$(VERSION)"
GO := go

.PHONY: all build clean dev test help \
        frontend-dev frontend-build frontend-test frontend-lint frontend-install \
        backend-build backend-test backend-lint backend-fmt \
        docker docker-compose-up docker-compose-down

all: build

#------------------------------------------------------------------------------
# Combined Targets
#------------------------------------------------------------------------------

build: backend-build frontend-build ## Build everything
	@echo "✅ Build complete!"

dev: ## Run both frontend and backend in dev mode
	@echo "Starting development servers..."
	@echo "Backend will run on :8081, Frontend on :5173"
	@trap 'kill 0' EXIT; \
		(cd backend && AIRGAPPER_PORT=8081 $(GO) run ./cmd/airgapper serve) & \
		(cd frontend && npm run dev) & \
		wait

test: backend-test frontend-test ## Run all tests
	@echo "✅ All tests passed!"

clean: ## Clean all build artifacts
	rm -rf bin/
	rm -rf backend/bin/
	rm -rf frontend/dist/
	rm -rf frontend/node_modules/.vite/
	rm -f coverage.out coverage.html
	@echo "✅ Cleaned!"

#------------------------------------------------------------------------------
# Frontend Targets
#------------------------------------------------------------------------------

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

frontend-dev: ## Run frontend dev server
	cd frontend && npm run dev

frontend-build: ## Build frontend for production
	cd frontend && npm run build

frontend-test: ## Run frontend tests
	cd frontend && npm run test

frontend-lint: ## Lint frontend code
	cd frontend && npm run lint

#------------------------------------------------------------------------------
# Backend Targets
#------------------------------------------------------------------------------

backend-build: ## Build Go binary
	cd backend && $(GO) build $(LDFLAGS) -o ../bin/$(BINARY) ./cmd/airgapper

backend-test: ## Run Go tests
	cd backend && $(GO) test -v ./...

backend-test-cover: ## Run Go tests with coverage
	cd backend && $(GO) test -v -coverprofile=coverage.out ./...
	cd backend && $(GO) tool cover -html=coverage.out -o coverage.html

backend-lint: ## Run Go linters
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	cd backend && golangci-lint run

backend-fmt: ## Format Go code
	cd backend && $(GO) fmt ./...

backend-install: backend-build ## Install binary to $GOPATH/bin
	cd backend && $(GO) install $(LDFLAGS) ./cmd/airgapper

#------------------------------------------------------------------------------
# Docker
#------------------------------------------------------------------------------

docker: ## Build Docker image
	docker build -t airgapper:latest -f docker/Dockerfile .

docker-compose-up: ## Start docker-compose stack
	docker-compose up -d

docker-compose-down: ## Stop docker-compose stack
	docker-compose down

docker-compose-logs: ## View docker-compose logs
	docker-compose logs -f

#------------------------------------------------------------------------------
# Development Helpers
#------------------------------------------------------------------------------

demo-init: ## Initialize demo environment
	@echo "Starting restic-rest-server..."
	docker run -d --name restic-rest-server -p 8000:8000 \
		-v restic-data:/data restic/rest-server --append-only --no-auth || true
	@echo ""
	@echo "REST server running at http://localhost:8000"
	@echo ""
	@echo "Initialize Airgapper:"
	@echo "  ./bin/airgapper init --name alice --repo rest:http://localhost:8000/mybackup"

demo-stop: ## Stop demo environment
	docker stop restic-rest-server || true
	docker rm restic-rest-server || true

#------------------------------------------------------------------------------
# Help
#------------------------------------------------------------------------------

help: ## Show this help
	@echo "Airgapper - Consensus-based encrypted backup system"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
