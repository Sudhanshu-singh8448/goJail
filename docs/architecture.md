# Architecture

goboxd is a Go HTTP service that runs untrusted code inside nsjail sandboxes and returns per-test results. Every `POST /run` request creates a fresh sandbox, optionally compiles the source, runs it against each test case, captures the output, and tears the sandbox down.

## High-Level Flow

```text
                  ┌─────────────────────────────────────────────────┐
   HTTP POST ──▶  │  chi Router (cmd/server/main.go)                │
   /run           │    ├── middleware: RequestID, RealIP, Recoverer │
                  │    └── HandleRun (internal/api/handlers.go)     │
                  │          ├── Parse JSON request                 │
                  │          ├── Validate (validation.go)           │
                  │          ├── Pool.SubmitAndWait ──────┐         │
                  │          └── Return JSON response     │         │
                  └───────────────────────────────────────┼─────────┘
                                                         │
                              ┌───────────────────────────▼──────┐
                              │  Worker Pool (worker/pool.go)    │
                              │    Bounded semaphore channel     │
                              │    Blocks when full (queues)     │
                              └───────────┬──────────────────────┘
                                          │
                      ┌───────────────────▼───────────────────┐
                      │  Sandbox (sandbox/sandbox.go)         │
                      │    ├── os.MkdirTemp (unique dir)      │
                      │    ├── WriteSource (validated path)   │
                      │    ├── Build (if compiled language)   │
                      │    ├── RunTest × N (concurrent)       │
                      │    └── defer Cleanup()                │
                      └───────────┬───────────────────────────┘
                                  │ exec.Command
                                  ▼
                  ┌────────────────────────────────────┐
                  │  nsjail (Linux namespaces)         │
                  │    ├── PID / mount / user ns       │
                  │    ├── rlimit_as / rlimit_nproc    │
                  │    ├── time_limit                  │
                  │    ├── bindmount (ro: /bin, /usr)  │
                  │    └── exec compiler or runtime    │
                  └────────────────────────────────────┘
```

## Components

### `cmd/server/main.go`

Entry point. Loads YAML configuration, initializes the worker pool, sets up the chi router with middleware, starts the orphan sweep goroutine, and listens for HTTP.

### `internal/api/`

The HTTP layer:

- **types.go** — Request/response structs matching the JSON API contract.
- **handlers.go** — Handlers for `/healthz`, `/readyz`, `/info`, and `POST /run`. The run handler parses the request, validates it, submits it to the worker pool, and returns the result.
- **validation.go** — Input validation: filename safety, flag allow-lists, size limits, language checks. Contains the primary security enforcement.

### `internal/sandbox/`

The execution layer:

- **sandbox.go** — `Sandbox` manages a single execution: creates a unique working directory with `os.MkdirTemp`, writes source code, runs build and test steps, and cleans up. All filesystem ops use Go stdlib, never shell commands.
- **nsjail.go** — Builds nsjail command-line args programmatically, runs nsjail via `exec.Command`, reads bounded output via `readBounded()`.
- **status.go** — Maps nsjail exit codes and stderr patterns to the API status vocabulary (`accepted`, `wrong_output`, `time_exceeded`, etc.).

### `internal/worker/`

- **pool.go** — Bounded concurrency pool using a buffered channel as a semaphore. `SubmitAndWait` blocks until a slot is available, runs the job, and returns the result. Jobs queue rather than fail when at capacity.

### `internal/config/`

- **config.go** — YAML-based configuration for server settings and language registry. Validates config at startup.

### `internal/types/`

- **types.go** — Shared data types (TestCase, BuildResult, TestResult, LimitsOverride) used by both the API and sandbox packages.

### `internal/logger/`

- **logger.go** — Structured JSON logging via `slog`.

### `config/`

- **server.yaml** — Server-level limits, paths, concurrency settings.
- **languages.yaml** — Language registry. Each language defines its build/run commands, default limits, filename patterns, smoke probes, and flag allow-lists.

## Request Lifecycle

1. **Parse**: JSON body decoded into `RunRequest`.
2. **Validate**: Language, source, filenames, flags, sizes all checked. Returns 400 on failure.
3. **Queue**: The request enters the worker pool's semaphore channel. If all slots are taken, it blocks.
4. **Create sandbox**: `os.MkdirTemp` creates a unique directory under the jail root.
5. **Write source**: Source code written to the sandbox directory. Path validated to prevent traversal.
6. **Build** (if needed): Compiler invoked inside nsjail with resource limits. Output captured with size caps.
7. **Run tests**: Each test runs concurrently in a goroutine. nsjail wraps each invocation with its own resource limits.
8. **Compare output**: Actual stdout compared against expected. Status assigned (`accepted`, `wrong_output`, `output_whitespace_mismatch`).
9. **Aggregate**: Overall status is `accepted` only if build succeeded and all tests passed. Otherwise it's the first non-accepted status.
10. **Cleanup**: `defer sb.Cleanup()` removes the sandbox directory on every exit path.
11. **Response**: JSON response returned to the client.

## Concurrency

Two levels:

- **Across requests**: The worker pool (`worker.Pool`) limits concurrent sandbox executions. Default is `runtime.NumCPU()`. When full, requests queue.
- **Within a request**: Test cases for a single request run concurrently in goroutines inside the same sandbox directory.

## Configuration

All configuration is YAML-based:

- `config/server.yaml` — port, limits, paths
- `config/languages.yaml` — language definitions

Environment variable overrides: `PORT`, `MAX_CONCURRENT_JOBS`, `CONFIG_DIR`.
