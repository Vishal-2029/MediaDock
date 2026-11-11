.DEFAULT_GOAL := help

BE_DIR := mediadock-BE
FE_DIR := mediadock-FE
COMPOSE_FILE := docker-compose.yml

ifeq ($(OS),Windows_NT)
	EXE := .exe
else
	EXE :=
endif

.PHONY: help setup-env deps deps-be deps-fe run dev dev-be dev-fe build build-be build-fe \
        test test-be test-fe lint vet verify docker-up docker-down docker-build docker-logs clean

help: ## Show available MediaDock make targets
	$(info MediaDock Development & Deployment Targets:)
	$(info   make setup-env     Create root .env from .env.example if missing)
	$(info   make deps          Install Go and npm dependencies)
	$(info   make run           Run both mediadock-BE (:8080) and mediadock-FE (:5173) locally without Docker)
	$(info   make dev           Alias for make run)
	$(info   make dev-be        Run mediadock-BE API server locally (port 8080))
	$(info   make dev-fe        Run mediadock-FE Vite dev server locally (port 5173))
	$(info   make build         Build both backend binary and frontend bundle)
	$(info   make build-be      Build mediadock-BE Go binary)
	$(info   make build-fe      Type-check and build mediadock-FE Vite bundle)
	$(info   make test          Run all backend and frontend automated tests)
	$(info   make test-be       Run mediadock-BE Go test suite)
	$(info   make test-fe       Run mediadock-FE Vitest test suite)
	$(info   make lint          Run TypeScript type-check and go vet)
	$(info   make verify        Run lint, vet, test, and build across BE and FE)
	$(info   make docker-up     Start Postgres, Redis, API, and Web via Docker Compose)
	$(info   make docker-down   Stop Docker Compose services)
	$(info   make docker-build  Rebuild and start all Docker Compose services)
	$(info   make docker-logs   Tail Docker Compose service logs)
	$(info   make clean         Remove generated build artifacts)
ifeq ($(OS),Windows_NT)
	@rem
else
	@true
endif

setup-env: ## Copy .env.example to .env if .env does not exist
ifeq ($(OS),Windows_NT)
	@if not exist .env (copy .env.example .env > nul && echo Created .env from .env.example) else (echo .env already exists)
else
	@if [ ! -f .env ]; then cp .env.example .env && echo "Created .env from .env.example"; else echo ".env already exists"; fi
endif

deps: deps-be deps-fe ## Install dependencies for both backend and frontend

deps-be: ## Download Go module dependencies
	cd $(BE_DIR) && go mod download

deps-fe: ## Install frontend npm dependencies
	cd $(FE_DIR) && npm install

run: setup-env ## Run both backend and frontend concurrently without Docker
	$(MAKE) -j2 dev-be dev-fe

dev: run ## Alias for run

dev-be: ## Run Go Fiber API server in development mode
	cd $(BE_DIR) && go run ./cmd/api/main.go

dev-fe: ## Run React + Vite development server
	cd $(FE_DIR) && npm run dev

build: build-be build-fe ## Build both backend and frontend artifacts

build-be: ## Build mediadock-BE binary
	cd $(BE_DIR) && go build -o bin/mediadock-api$(EXE) ./cmd/api

build-fe: ## Build mediadock-FE production assets
	cd $(FE_DIR) && npm run build

test: test-be test-fe ## Run all backend and frontend tests

test-be: ## Run mediadock-BE test suite
	cd $(BE_DIR) && go test -timeout 30s -v ./...

test-fe: ## Run mediadock-FE unit & component tests
	cd $(FE_DIR) && npm test

vet: ## Run go vet on mediadock-BE
	cd $(BE_DIR) && go vet ./...

lint: vet ## Run TypeScript checks and go vet
	cd $(FE_DIR) && npm run lint

verify: lint test build ## Full local verification pipeline (lint, test, build)

docker-up: setup-env ## Start full stack with Docker Compose
	docker-compose -f $(COMPOSE_FILE) up -d

docker-build: setup-env ## Rebuild and start full stack with Docker Compose
	docker-compose -f $(COMPOSE_FILE) up -d --build

docker-down: ## Stop Docker Compose stack
	docker-compose -f $(COMPOSE_FILE) down

docker-logs: ## Follow logs from all Docker Compose services
	docker-compose -f $(COMPOSE_FILE) logs -f

clean: ## Clean local build outputs
ifeq ($(OS),Windows_NT)
	@if exist "$(BE_DIR)\bin" rmdir /s /q "$(BE_DIR)\bin"
	@if exist "$(FE_DIR)\dist" rmdir /s /q "$(FE_DIR)\dist"
else
	rm -rf $(BE_DIR)/bin $(FE_DIR)/dist
endif
