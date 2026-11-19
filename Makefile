.PHONY: help up down logs ps build test test-docker cover lint fmt e2e

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

up: ## Build and start the whole stack
	@test -f .env || cp .env.example .env
	docker compose up -d --build

down: ## Stop the stack and remove volumes
	docker compose down -v

logs: ## Follow logs of every service
	docker compose logs -f

ps: ## Show service status
	docker compose ps

build: ## Compile every service
	go build ./...

test: ## Run the unit test suite
	go test -race -count=1 ./...

test-docker: ## Run the test suite inside the official Go image (no local Go needed)
	./scripts/go-docker.sh go test -count=1 ./...

cover: ## Run tests with coverage
	go test -count=1 -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

lint: ## Run go vet and golangci-lint
	go vet ./...
	golangci-lint run ./...

fmt: ## Format the code
	gofmt -w .

e2e: ## Run the end-to-end smoke test against a running stack
	./scripts/e2e.sh
