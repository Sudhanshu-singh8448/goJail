# Development Guide

## Prerequisites

- Docker (Engine 24+) with Docker Compose v2
- Go 1.23+ (for local development and testing)
- A Linux host or Docker Desktop with Linux containers (nsjail uses Linux namespaces)

## Running

### Docker (recommended)

```bash
make build    # Build the Docker image
make run      # Start the server (foreground)
```

The server listens on `http://localhost:8000`.

### Local (requires nsjail on host)

```bash
go run ./cmd/server/
```

## Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build Docker image |
| `make build-no-cache` | Build from scratch |
| `make run` | Start server (foreground) |
| `make run-detached` | Start server (background) |
| `make down` | Stop and clean up |
| `make test` | Run unit tests |
| `make test-verbose` | Run unit tests with verbose output |
| `make integration` | Run integration tests (Docker running) |
| `make load` | Run load tests (Docker running) |
| `make lint` | go vet + staticcheck |
| `make vet` | go vet only |
| `make fmt` | Format code |
| `make tidy` | go mod tidy |
| `make shell` | Open a shell inside the container |
| `make build-local` | Build binary locally |

## Testing

### Unit Tests

```bash
make test
```

Tests cover:
- YAML config loading and validation
- Filename validation (path traversal prevention)
- Flag allow-list matching
- Request validation
- Status parsing and output comparison
- Bounded output reading

### Integration Tests

```bash
make build && make run-detached
make integration
```

Tests all endpoints and all 7 languages against a running Docker container.

### Load Tests

```bash
make build && make run-detached
make load
```

Runs at 1, 10, 50, and 100 concurrent clients. Results go into `docs/benchmarks.md`.

## Adding a Language

See [docs/languages.md](languages.md) for the step-by-step guide. No Go code change needed.

## Code Quality

```bash
make lint    # go vet + staticcheck
make fmt     # gofmt
```

## Project Structure

```
goboxd/
├── cmd/server/          # HTTP server entry point
├── internal/
│   ├── api/             # HTTP handlers, types, validation
│   ├── config/          # YAML config loading
│   ├── logger/          # Structured JSON logging
│   ├── sandbox/         # nsjail sandbox lifecycle
│   ├── types/           # Shared data types
│   └── worker/          # Bounded concurrency pool
├── config/              # YAML configuration files
├── scripts/             # Build and install scripts
├── tests/               # Integration and load tests
└── docs/                # Documentation
```
