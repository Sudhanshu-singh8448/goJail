# Security

The reference Python implementation has 7 known security holes. This document lists each hole, its risk, and where goboxd closes it.

## Holes Closed: 7/7

### 1. Path traversal via filename

**Risk**: Client supplies `../../etc/passwd` as `source_filename`, writing outside the jail directory.

**Fix**: `ValidateFilename()` in [internal/api/validation.go](../internal/api/validation.go) rejects any filename containing `/`, `\`, `..`, a leading `.`, null bytes, or exceeding 255 characters. `filepath.Base()` is checked as a final guard. Additionally, `WriteSource()` in [internal/sandbox/sandbox.go](../internal/sandbox/sandbox.go) verifies the resolved absolute path starts with the sandbox directory prefix.

### 2. Shell-style directory commands

**Risk**: The reference formats shell commands to create/delete directories, enabling injection.

**Fix**: All filesystem operations in [internal/sandbox/sandbox.go](../internal/sandbox/sandbox.go) use Go stdlib: `os.MkdirAll`, `os.MkdirTemp`, `os.WriteFile`, `os.RemoveAll`. Zero shell invocations for any filesystem operation anywhere in the codebase.

### 3. Compiler-flag injection

**Risk**: Arbitrary flags like `-fplugin=...`, `-x c`, `-B...`, `--specs=...`, `@response_file` give compile-time code execution on the host.

**Fix**: `ValidateFlags()` in [internal/api/validation.go](../internal/api/validation.go) checks every flag against the per-language `flag_allowlist` defined in `config/languages.yaml`. Glob matching supports patterns like `-std=c++*`. Any flag not in the list returns HTTP 400.

### 4. No request size limits

**Risk**: Unbounded source, test count, stdin, and stdout can OOM the server.

**Fix**: Multiple layers in [internal/api/handlers.go](../internal/api/handlers.go) and [internal/api/validation.go](../internal/api/validation.go):
- `http.MaxBytesReader` on the request body
- Source size check against `max_source_bytes` (default 256 KiB)
- Test count check against `max_tests` (default 50)
- Per-test stdin and expected_stdout size checks
- All limits configurable in `config/server.yaml`

### 5. UID collisions under load

**Risk**: The reference picks a random UID from a 30k range and retries 3 times. Under load, collisions cause directory conflicts.

**Fix**: `NewSandbox()` in [internal/sandbox/sandbox.go](../internal/sandbox/sandbox.go) uses `os.MkdirTemp` which creates a unique directory atomically using the OS's tempdir mechanism. No UID generation, no retry loop, no collision possible.

### 6. Unbounded child output

**Risk**: A malicious program prints gigabytes of output, OOMing the host process.

**Fix**: `readBounded()` in [internal/sandbox/nsjail.go](../internal/sandbox/nsjail.go) uses `io.ReadFull` with a buffer of `maxBytes+1` to detect and cap output. Truncated output gets a `\n[truncated]` marker. The remaining pipe is drained to `io.Discard` to prevent child process blocking. Limits configurable via `max_stdout_capture_bytes` and `max_stderr_capture_bytes` in `config/server.yaml`.

### 7. Stale jail directories

**Risk**: A panic between directory creation and cleanup leaks the directory, filling disk.

**Fix**: Three layers:
1. `defer sb.Cleanup()` in `executeRun()` at [internal/api/handlers.go](../internal/api/handlers.go) — runs on every exit path including panics (chi's `Recoverer` middleware catches panics at the HTTP layer).
2. `Cleanup()` in [internal/sandbox/sandbox.go](../internal/sandbox/sandbox.go) uses `sync.Mutex` to be safe for multiple calls.
3. `SweepOrphanJails()` in [internal/sandbox/sandbox.go](../internal/sandbox/sandbox.go) runs at startup and periodically (configurable via `orphan_sweep_minutes`) to remove directories older than the sweep age.
