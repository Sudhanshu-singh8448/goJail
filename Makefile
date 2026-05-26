.PHONY: build run test integration load lint vet fmt tidy down shell build-no-cache

.DEFAULT_GOAL := run

# Build the Docker image
build:
	@docker compose build goboxd

# Build without cache
build-no-cache:
	@docker compose build goboxd --no-cache

# Run the server via Docker Compose
run:
	@docker compose up

# Run in detached mode
run-detached:
	@docker compose up -d

# Stop the server
down:
	@docker compose down -v --remove-orphans

# Run unit tests (no Docker needed)
test:
	@go test ./... -timeout 30s

# Run unit tests with verbose output
test-verbose:
	@go test ./... -v -timeout 30s

# Run integration tests (requires Docker running)
integration:
	@bash tests/integration_test.sh

# Run load test (requires Docker running)
load:
	@bash tests/load_test.sh

# Lint the code
lint:
	@go vet ./...
	@echo "go vet: OK"
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
		echo "staticcheck: OK"; \
	else \
		echo "staticcheck not installed, skipping (install with: go install honnef.co/go/tools/cmd/staticcheck@latest)"; \
	fi

# Go vet only
vet:
	@go vet ./...

# Format code
fmt:
	@gofmt -w .

# Tidy modules
tidy:
	@go mod tidy

# Open a shell inside the container
shell:
	@docker compose run --rm goboxd bash

# Build the binary locally (for development)
build-local:
	@go build -o goboxd ./cmd/server/

# Run locally (requires nsjail on host)
run-local:
	@go run ./cmd/server/
