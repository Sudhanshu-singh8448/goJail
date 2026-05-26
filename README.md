# goboxd

A Go HTTP service that runs untrusted code inside nsjail sandboxes and returns per-test results.

## Framework

Uses [chi](https://github.com/go-chi/chi) — a lightweight HTTP router compatible with `net/http`, with middleware support and no code generation.

## Quick Start

```bash
make build
make run
curl http://localhost:8000/healthz
```

The server listens on `http://localhost:8000`.

## Supported Languages

| ID         | Language             | Compiled |
| ---------- | -------------------- | -------- |
| `py3`      | Python 3             | no       |
| `cpp`      | C++                  | yes      |
| `c`        | C                    | yes      |
| `java`     | Java (OpenJDK 17)    | yes      |
| `bash`     | Bash                 | no       |
| `javascript` | Node.js           | no       |
| `verilog`  | Verilog (Icarus)     | yes      |

## Documentation

- [API](docs/api.md) — request/response contract
- [Languages](docs/languages.md) — language registry, adding a new language
- [Architecture](docs/architecture.md) — system design
- [Security](docs/security.md) — security holes and fixes
- [Benchmarks](docs/benchmarks.md) — load test results
- [Development](docs/development.md) — dev workflow

## Make Targets

| Target | Description |
| ------ | ----------- |
| `make build` | Build Docker image |
| `make run` | Start the server |
| `make test` | Run unit tests |
| `make integration` | Run integration tests |
| `make load` | Run load tests |
| `make lint` | Run go vet + staticcheck |
| `make fmt` | Format code |
| `make down` | Stop the server |
