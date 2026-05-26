# goboxd — Full Implementation Plan (Stages 1–3)

The existing codebase is a direct port of the Python/Flask pyjail server, using protobuf text format on the wire and protobuf-based config files. The hackathon spec requires a ground-up rewrite to a JSON API with YAML-driven language configuration, proper security, bounded concurrency, and health endpoints. The protobuf layer, evaluation-script mode, and Python 2 support are all out of scope and will be removed.

## Current State

- **Wire format**: Protobuf text format wrapped in JSON `{"message": "..."}` — needs to become plain JSON matching the spec.
- **Config**: Protobuf `.conf` files in `config/` — needs to become YAML in `config/languages.yaml`.
- **Languages**: 10 languages configured (including py2, haskell, zip) — needs to be trimmed to the 7 in-scope languages.
- **Security**: Zero of the 7 holes are closed.
- **Concurrency**: No bounded concurrency, no queue, no benchmarks.
- **Health endpoints**: Only `GET /` returning `"OK"` — needs `/healthz`, `/readyz`, `/info`.
- **Tests**: None.
- **nsjail**: Cloned from git at build time (correct), but not pinned as a submodule.

## User Review Required

> [!IMPORTANT]
> **Framework choice**: I'll use `chi` (lightweight, stdlib-compatible, no external deps beyond the router). It gives us middleware chaining, URL params, and structured route grouping without the weight of gin/echo. This keeps the README justification honest: "chi because it's a thin wrapper over net/http with middleware support and no code generation."

> [!IMPORTANT]
> **Complete rewrite**: The existing protobuf-based architecture (proto files, generated Go code, protobuf config parsing) will be entirely replaced. The `proto/` directory, `internal/settings/`, and the current `internal/codemanager/` and `internal/coderunner/` will be deleted and rewritten from scratch. The new code uses plain JSON request/response structs and YAML config.

> [!WARNING]
> **nsjail submodule**: The spec says to add nsjail as a git submodule. I'll run `git submodule add https://github.com/google/nsjail external/nsjail` and build it in the Dockerfile from that submodule. This changes the Dockerfile build step.

## Proposed Changes

The plan is organized by component. Every file is listed with its action (NEW/MODIFY/DELETE).

---

### 1. Project Root — Cleanup & Config

#### [DELETE] [proto/](file:///Users/sudhanshukumar/Desktop/gojail/proto)
Remove the entire proto directory (`.proto` files and generated Go code). No longer needed.

#### [DELETE] [config/public_settings.conf](file:///Users/sudhanshukumar/Desktop/gojail/config/public_settings.conf)
#### [DELETE] [config/private_settings.conf](file:///Users/sudhanshukumar/Desktop/gojail/config/private_settings.conf)
Replaced by YAML config.

#### [NEW] config/languages.yaml
YAML language registry. All 7 in-scope languages defined here. Shape:

```yaml
languages:
  - id: py3
    name: "Python 3"
    source_filename: solution.py
    smoke_cmd: ["/usr/bin/python3", "--version"]
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100

  - id: cpp
    name: "C++"
    source_filename: solution.cpp
    artifact_filename: solution
    smoke_cmd: ["/usr/bin/g++", "--version"]
    build:
      cmd: /usr/bin/g++
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits:
        wall_time_s: 3
        memory_kb: 1048576
        max_processes: 100
      flag_allowlist: ["-O0", "-O1", "-O2", "-O3", "-Wall", "-Wextra", "-std=c++11", "-std=c++14", "-std=c++17", "-std=c++20", "-lm"]
    run:
      cmd: "./{{artifact}}"
      limits:
        wall_time_s: 3
        memory_kb: 524288
        max_processes: 64

  - id: c
    name: "C"
    source_filename: solution.c
    artifact_filename: solution
    smoke_cmd: ["/usr/bin/gcc", "--version"]
    build:
      cmd: /usr/bin/gcc
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits:
        wall_time_s: 3
        memory_kb: 1048576
        max_processes: 100
      flag_allowlist: ["-O0", "-O1", "-O2", "-O3", "-Wall", "-Wextra", "-std=c99", "-std=c11", "-std=c17", "-lm"]
    run:
      cmd: "./{{artifact}}"
      limits:
        wall_time_s: 3
        memory_kb: 524288
        max_processes: 64

  - id: java
    name: "Java"
    source_filename_strategy: from_request
    artifact_filename_strategy: from_request
    smoke_cmd: ["/usr/bin/javac", "-version"]
    build:
      cmd: /usr/bin/javac
      args: ["{{flags}}", "{{source}}"]
      limits:
        wall_time_s: 6
        memory_kb: 102400
        max_processes: 100
      flag_allowlist: []
    run:
      cmd: /usr/bin/java
      args: ["{{artifact}}"]
      limits:
        wall_time_s: 6
        memory_kb: 102400
        max_processes: 100

  - id: bash
    name: "Bash"
    source_filename: solution.sh
    smoke_cmd: ["/bin/bash", "--version"]
    run:
      cmd: /bin/bash
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100

  - id: javascript
    name: "JavaScript (Node.js)"
    source_filename: solution.js
    smoke_cmd: ["/usr/bin/node", "--version"]
    run:
      cmd: /usr/bin/node
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100

  - id: verilog
    name: "Verilog"
    source_filename: solution.v
    artifact_filename: solution
    smoke_cmd: ["/usr/bin/iverilog", "-V"]
    build:
      cmd: /usr/bin/iverilog
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits:
        wall_time_s: 6
        memory_kb: 102400
        max_processes: 100
      flag_allowlist: []
    run:
      cmd: /usr/bin/vvp
      args: ["{{artifact}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
```

#### [NEW] config/server.yaml
Server-level configuration:

```yaml
server:
  port: 8000
  max_source_bytes: 262144        # 256 KiB
  max_tests: 50
  max_concurrent_jobs: 0          # 0 = runtime.NumCPU()
  max_stdin_bytes: 1048576        # 1 MiB per test stdin
  max_expected_bytes: 1048576     # 1 MiB per test expected
  max_stdout_capture_bytes: 1048576  # 1 MiB stdout cap
  max_stderr_capture_bytes: 262144   # 256 KiB stderr cap
  jail_dir: /tmp/goboxd_jails
  nsjail_path: /usr/bin/nsjail
  orphan_sweep_minutes: 5
```

#### [MODIFY] [go.mod](file:///Users/sudhanshukumar/Desktop/gojail/go.mod)
- Remove `google.golang.org/protobuf` dependency
- Add `github.com/go-chi/chi/v5`, `gopkg.in/yaml.v3`, `github.com/google/uuid`

---

### 2. Internal Packages — Complete Rewrite

#### [DELETE] [internal/settings/settings.go](file:///Users/sudhanshukumar/Desktop/gojail/internal/settings/settings.go)
#### [DELETE] [internal/codemanager/manager.go](file:///Users/sudhanshukumar/Desktop/gojail/internal/codemanager/manager.go)
#### [DELETE] [internal/coderunner/runner.go](file:///Users/sudhanshukumar/Desktop/gojail/internal/coderunner/runner.go)

These are replaced by the new packages below.

---

#### [MODIFY] [internal/config/config.go](file:///Users/sudhanshukumar/Desktop/gojail/internal/config/config.go)
Rewrite to load `server.yaml` + `languages.yaml`. Struct definitions:

```go
type ServerConfig struct {
    Port                  int    `yaml:"port"`
    MaxSourceBytes        int    `yaml:"max_source_bytes"`
    MaxTests              int    `yaml:"max_tests"`
    MaxConcurrentJobs     int    `yaml:"max_concurrent_jobs"`
    MaxStdinBytes         int    `yaml:"max_stdin_bytes"`
    MaxExpectedBytes      int    `yaml:"max_expected_bytes"`
    MaxStdoutCaptureBytes int    `yaml:"max_stdout_capture_bytes"`
    MaxStderrCaptureBytes int    `yaml:"max_stderr_capture_bytes"`
    JailDir               string `yaml:"jail_dir"`
    NsjailPath            string `yaml:"nsjail_path"`
    OrphanSweepMinutes    int    `yaml:"orphan_sweep_minutes"`
}

type Limits struct {
    WallTimeS    int `yaml:"wall_time_s"    json:"wall_time_s"`
    MemoryKB     int `yaml:"memory_kb"      json:"memory_kb"`
    MaxProcesses int `yaml:"max_processes"  json:"max_processes"`
}

type CommandConfig struct {
    Cmd           string   `yaml:"cmd"`
    Args          []string `yaml:"args"`
    Limits        Limits   `yaml:"limits"`
    FlagAllowlist []string `yaml:"flag_allowlist"`
}

type LanguageConfig struct {
    ID                       string         `yaml:"id"`
    Name                     string         `yaml:"name"`
    SourceFilename           string         `yaml:"source_filename"`
    ArtifactFilename         string         `yaml:"artifact_filename"`
    SourceFilenameStrategy   string         `yaml:"source_filename_strategy"`
    ArtifactFilenameStrategy string         `yaml:"artifact_filename_strategy"`
    SmokeCmd                 []string       `yaml:"smoke_cmd"`
    Build                    *CommandConfig `yaml:"build"`
    Run                      CommandConfig  `yaml:"run"`
}
```

Functions:
- `LoadServerConfig(path string) (*ServerConfig, error)`
- `LoadLanguages(path string) ([]LanguageConfig, error)`
- `FindLanguage(langs []LanguageConfig, id string) *LanguageConfig`

---

#### [NEW] internal/api/types.go
Request and response types matching the API contract:

```go
// Request types
type RunRequest struct {
    Language         string     `json:"language"`
    Source           string     `json:"source"`
    SourceFilename   string     `json:"source_filename"`
    ArtifactFilename string     `json:"artifact_filename"`
    Build            *PhaseOpts `json:"build"`
    Run              *PhaseOpts `json:"run"`
    Tests            []TestCase `json:"tests"`
}

type PhaseOpts struct {
    Limits *Limits  `json:"limits"`
    Flags  []string `json:"flags"`
}

type TestCase struct {
    Stdin          string `json:"stdin"`
    ExpectedStdout string `json:"expected_stdout"`
}

// Response types
type RunResponse struct {
    Status string          `json:"status"`
    Build  *BuildResult    `json:"build,omitempty"`
    Tests  []TestResult    `json:"tests"`
}

type BuildResult struct {
    Status     string `json:"status"`
    Stdout     string `json:"stdout"`
    Stderr     string `json:"stderr"`
    DurationMs int64  `json:"duration_ms"`
}

type TestResult struct {
    Status       string `json:"status"`
    Stdout       string `json:"stdout"`
    Stderr       string `json:"stderr"`
    DurationMs   int64  `json:"duration_ms"`
    MemoryPeakKB int64  `json:"memory_peak_kb"`
}

type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

---

#### [NEW] internal/api/handlers.go
HTTP handlers for all endpoints:

- `HandleHealthz` — `GET /healthz`, returns `{"status":"ok"}`
- `HandleReadyz` — `GET /readyz`, checks nsjail binary + each language's smoke command
- `HandleInfo` — `GET /info`, returns build info, nsjail info, registered languages, limits, stats
- `HandleRun` — `POST /run`, the main execution endpoint

---

#### [NEW] internal/api/validation.go
Request validation logic:

- `validateRunRequest(req *RunRequest, cfg *ServerConfig, langs []LanguageConfig) *ErrorResponse`
- `validateFilename(name string) error` — single path component, no separators, no leading dot, length ≤ 255
- `validateFlags(flags []string, allowlist []string) error` — glob matching for `*` entries like `-std=*`
- Source size check against `max_source_bytes`
- Test count check against `max_tests`
- Per-test stdin/expected size checks

---

#### [NEW] internal/sandbox/sandbox.go
Core sandbox lifecycle:

```go
type Sandbox struct {
    ID        string
    WorkDir   string
    Language  *LanguageConfig
    NsjailPath string
    ServerCfg  *ServerConfig
}

func NewSandbox(lang *LanguageConfig, nsjailPath string, cfg *ServerConfig) (*Sandbox, error)
func (s *Sandbox) WriteSource(source string, filename string) error
func (s *Sandbox) Build(limits Limits, flags []string) (*BuildResult, error)
func (s *Sandbox) RunTest(test TestCase, limits Limits, flags []string) (*TestResult, error)
func (s *Sandbox) Cleanup()
```

Key design decisions:
- Uses `os.MkdirTemp` for unique directories (no UID collision — fixes hole #5)
- `defer s.Cleanup()` at the right scope (fixes hole #7)
- All paths validated with `filepath.Abs` + prefix check (fixes holes #1, #2)
- No shell commands for directory ops — uses `os.MkdirAll`, `os.RemoveAll` (fixes hole #2)
- Bounded stdout/stderr reads (fixes hole #6)

---

#### [NEW] internal/sandbox/nsjail.go
nsjail command building and execution:

```go
func buildNsjailArgs(nsjailPath string, workDir string, limits Limits, isCompile bool) []string
func runNsjail(args []string, stdin io.Reader, maxStdout, maxStderr int) (stdout, stderr []byte, duration time.Duration, err error)
func parseNsjailLog(logPath string) (memoryPeakKB int64, err error)
```

The nsjail args are built programmatically (not from template strings), using:
```
nsjail --mode once_only --chroot / --cwd <workdir>
  --bindmount_ro /bin --bindmount_ro /usr --bindmount_ro /lib --bindmount_ro /lib64
  --bindmount_ro /etc --bindmount_ro /dev
  --bindmount <workdir>
  --time_limit <wall_time_s>
  --rlimit_as <memory_kb * 1024>
  --rlimit_nproc <max_processes>
  --rlimit_fsize 100
  --rlimit_nofile 1000
  --max_cpus 1
  --disable_proc
  --log <workdir>/nsjail.log
  -- <cmd> <args...>
```

---

#### [NEW] internal/sandbox/status.go
Status parsing and mapping:

```go
func parseBuildStatus(exitCode int, stderr string) string
func parseTestStatus(exitCode int, stderr string, nsjailLog string) string
func compareOutput(actual, expected string) string  // "accepted", "wrong_output", "output_whitespace_mismatch"
func computeOverallStatus(buildStatus string, testResults []TestResult) string
```

Status vocabulary matches the spec exactly.

---

#### [NEW] internal/worker/pool.go
Bounded concurrency with a semaphore + queue:

```go
type Pool struct {
    sem     chan struct{}
    mu      sync.Mutex
    inFlight int64
    total    int64
    failedInternal int64
    lastInternalErr time.Time
}

func NewPool(maxConcurrent int) *Pool
func (p *Pool) Submit(ctx context.Context, fn func() error) error  // blocks until slot available
func (p *Pool) Stats() PoolStats
```

When `maxConcurrent` slots are all taken, `Submit` blocks (queues). It does not fail.

---

#### [NEW] internal/stats/stats.go
Global statistics tracking:

```go
type Stats struct {
    InFlightJobs      int64     `json:"in_flight_jobs"`
    JobsTotal         int64     `json:"jobs_total"`
    JobsFailedInternal int64   `json:"jobs_failed_internal"`
    LastInternalErrorAt *time.Time `json:"last_internal_error_at,omitempty"`
    DiskFreeBytesJailDir int64 `json:"disk_free_bytes_jail_dir"`
}
```

---

#### [MODIFY] [internal/logger/logger.go](file:///Users/sudhanshukumar/Desktop/gojail/internal/logger/logger.go)
Keep but enhance: switch to JSON structured logging. One JSON line per request with request_id, language, durations, status.

---

### 3. Server Entry Point

#### [MODIFY] [cmd/server/main.go](file:///Users/sudhanshukumar/Desktop/gojail/cmd/server/main.go)
Complete rewrite:

```go
func main() {
    // Load config
    // Load languages from YAML
    // Validate all languages at startup (smoke probes)
    // Create worker pool
    // Start orphan sweep goroutine
    // Set up chi router:
    //   GET  /healthz  → HandleHealthz
    //   GET  /readyz   → HandleReadyz
    //   GET  /info     → HandleInfo
    //   POST /run      → HandleRun
    // Listen and serve
}
```

---

### 4. Docker & Build

#### [MODIFY] [Dockerfile](file:///Users/sudhanshukumar/Desktop/gojail/Dockerfile)
Major rewrite:
- Multi-stage build
- Build nsjail from `external/nsjail` submodule (not git clone)
- Install all 7 language toolchains
- Copy `config/` with YAML files
- Build Go binary with ldflags for version/commit injection
- Final stage: slim image with nsjail + toolchains + Go binary

#### [MODIFY] [docker-compose.yml](file:///Users/sudhanshukumar/Desktop/gojail/docker-compose.yml)
Add environment variables for config, expose proper ports.

#### [MODIFY] [Makefile](file:///Users/sudhanshukumar/Desktop/gojail/Makefile)
Complete rewrite with required targets:
- `make build` — docker compose build
- `make run` — docker compose up
- `make test` — go test ./...
- `make integration` — run integration tests against docker
- `make load` — run load test script
- `make lint` — go vet + staticcheck

#### [MODIFY] [entrypoint.sh](file:///Users/sudhanshukumar/Desktop/gojail/entrypoint.sh)
Set up JAVA_HOME, create jail directory, sweep orphans, exec the binary.

---

### 5. Scripts

#### [MODIFY] [scripts/install.sh](file:///Users/sudhanshukumar/Desktop/gojail/scripts/install.sh)
Simplify: just install system deps for nsjail build. Language installs are per-script.

#### [DELETE] scripts/lang_install/python.sh
Python 2 is out of scope.

#### [DELETE] scripts/lang_install/postgres.sh
PostgreSQL/evaluation mode is out of scope.

The remaining scripts (c.sh, cpp.sh, java.sh, javascript.sh, python3.sh, verilog.sh) stay but get cleaned up.

#### [NEW] scripts/lang_install/bash.sh
Bash is already present on Debian but add a smoke test script.

---

### 6. Tests

#### [NEW] internal/config/config_test.go
Unit tests for YAML config loading, language lookup.

#### [NEW] internal/api/validation_test.go
Unit tests for:
- Filename validation (path traversal, leading dots, separators)
- Flag allow-list matching (glob patterns, rejections)
- Source size limits
- Test count limits

#### [NEW] internal/sandbox/status_test.go
Unit tests for:
- Output comparison (exact match, whitespace mismatch, wrong output)
- Overall status computation
- Build status parsing

#### [NEW] internal/sandbox/sandbox_test.go
Unit tests for:
- Path construction (no traversal)
- Output truncation with marker

#### [NEW] tests/integration_test.go
End-to-end tests for each language. One `TestRunPy3HelloWorld`, `TestRunCppHelloWorld`, etc. These require Docker.

#### [NEW] tests/load_test.sh
Load test script using `hey`:
```bash
hey -n 1000 -c $CONCURRENCY -m POST \
  -H "Content-Type: application/json" \
  -D testdata/py3_hello.json \
  http://localhost:8000/run
```

---

### 7. Documentation

#### [MODIFY] [README.md](file:///Users/sudhanshukumar/Desktop/gojail/README.md)
Short, clean:
- What it is (one sentence)
- How to run (`make build && make run`)
- Framework choice (chi, one sentence)
- Where the docs are
- Languages supported (table)

#### [MODIFY] [docs/architecture.md](file:///Users/sudhanshukumar/Desktop/gojail/docs/architecture.md)
Rewrite for the new Go architecture. Request lifecycle, component diagram, concurrency model.

#### [NEW] docs/api.md
Full API contract documentation matching the spec.

#### [NEW] docs/languages.md
How the language registry works, how to add a new language, YAML schema reference.

#### [NEW] docs/security.md
The 7 security holes and where each is fixed (file:line).

#### [NEW] docs/benchmarks.md
Template for benchmark results (p50/p95/p99 at 1/10/50/100 clients).

#### [MODIFY] [docs/development.md](file:///Users/sudhanshukumar/Desktop/gojail/docs/development.md)
Update for new Makefile targets and Go-based workflow.

#### [DELETE] [docs/getting-started.md](file:///Users/sudhanshukumar/Desktop/gojail/docs/getting-started.md)
Merged into README.

---

### 8. Security Fixes (All 7)

| # | Hole | Fix Location | How |
|---|------|-------------|-----|
| 1 | Path traversal via filename | `internal/api/validation.go` | `validateFilename()` rejects any path with `/`, `\`, `..`, leading `.`, or length > 255. Uses `filepath.Base()` check. |
| 2 | Shell-style directory commands | `internal/sandbox/sandbox.go` | All directory operations use `os.MkdirAll`, `os.MkdirTemp`, `os.RemoveAll`. Zero shell invocations for filesystem ops. |
| 3 | Compiler-flag injection | `internal/api/validation.go` | `validateFlags()` checks every flag against the per-language `flag_allowlist`. Glob matching for patterns like `-std=*`. Rejects with 400. |
| 4 | No request size limits | `internal/api/handlers.go` + `validation.go` | `http.MaxBytesReader` on the request body. Source size, test count, per-test stdin/expected size all checked. |
| 5 | UID collisions under load | `internal/sandbox/sandbox.go` | Uses `os.MkdirTemp` which creates a unique directory atomically. No UID generation, no retry loop. |
| 6 | Unbounded child output | `internal/sandbox/nsjail.go` | `io.LimitedReader` on stdout/stderr pipes. Truncated output gets a `[truncated]` marker appended. |
| 7 | Stale jail directories | `internal/sandbox/sandbox.go` + `cmd/server/main.go` | `defer sandbox.Cleanup()` at the right scope. Startup sweep goroutine removes directories older than N minutes. |

---

## Verification Plan

### Automated Tests

```bash
# Unit tests (no Docker needed)
make test

# Lint
make lint

# Docker build
make build

# Health check
curl http://localhost:8000/healthz

# Integration tests (Docker running)
make integration

# Load test
make load
```

### Manual Verification

1. **Health endpoints**: `curl /healthz`, `/readyz`, `/info` — verify JSON format matches spec
2. **Each language**: Send a "Hello World" POST /run for py3, cpp, c, java, bash, javascript, verilog
3. **Error cases**: Bad JSON, unknown language, oversize source, path traversal filename, disallowed flag
4. **Security**: Attempt `../../etc/passwd` filename, `-fplugin=...` flag, oversized output
5. **Concurrency**: Run load test at 1, 10, 50, 100 clients and record benchmarks
6. **Demo-day language add**: Add Rust or Go as a new language in < 30 min with only YAML + Dockerfile changes

### Test Data Files

#### [NEW] tests/testdata/py3_hello.json
```json
{
  "language": "py3",
  "source": "print('hello')",
  "tests": [{"stdin": "", "expected_stdout": "hello\n"}]
}
```

Similar files for each language.
