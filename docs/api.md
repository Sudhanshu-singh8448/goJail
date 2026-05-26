# API Contract

## POST /run

Execute untrusted code in a sandboxed environment and compare output against test cases.

### Request

```json
{
  "language": "cpp",
  "source": "#include <iostream>\nint main(){std::cout<<\"hi\";}",
  "source_filename": "solution.cpp",
  "artifact_filename": "solution",
  "build": {
    "limits": { "wall_time_s": 5, "memory_kb": 1048576, "max_processes": 100 },
    "flags": ["-O2"]
  },
  "run": {
    "limits": { "wall_time_s": 3, "memory_kb": 524288, "max_processes": 64 },
    "flags": []
  },
  "tests": [
    { "stdin": "1\n", "expected_stdout": "hi" }
  ]
}
```

#### Field Rules

| Field | Required | Notes |
|-------|----------|-------|
| `language` | yes | Must match a configured language id |
| `source` | yes | UTF-8, max 256 KiB |
| `source_filename` | depends | Required for languages with `source_filename_strategy: from_request` (e.g. Java) |
| `artifact_filename` | depends | Required for languages with `artifact_filename_strategy: from_request` |
| `build` | no | Overrides language defaults for the build phase |
| `run` | no | Overrides language defaults for the run phase |
| `build.limits` / `run.limits` | no | Partial override — missing fields fall back to defaults |
| `build.flags` / `run.flags` | no | Must pass the per-language allow-list or get 400 |
| `tests` | yes | At least 1, at most 50 |

### Response (200)

```json
{
  "status": "wrong_output",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 412
  },
  "tests": [
    {
      "status": "wrong_output",
      "stdout": "HI",
      "stderr": "",
      "duration_ms": 38,
      "memory_peak_kb": 8192
    }
  ]
}
```

### Error Response (400)

```json
{
  "error": {
    "code": "invalid_filename",
    "message": "source_filename must be a single path component"
  }
}
```

### Error Codes

| Code | Meaning |
|------|---------|
| `invalid_json` | Malformed JSON body |
| `missing_language` | `language` field missing |
| `unknown_language` | Language not configured |
| `missing_source` | `source` field missing |
| `invalid_source` | Source is not valid UTF-8 |
| `source_too_large` | Source exceeds max size |
| `missing_tests` | No tests provided |
| `too_many_tests` | Too many tests |
| `stdin_too_large` | Test stdin too large |
| `expected_stdout_too_large` | Test expected_stdout too large |
| `invalid_filename` | Path traversal or invalid filename |
| `missing_filename` | Required filename not provided |
| `disallowed_flag` | Flag not in allow-list |
| `invalid_flags` | Flags given for a language without build step |

## Status Vocabulary

| Scope | Values |
|-------|--------|
| `build.status` | `ok`, `failed`, `internal_error` |
| `test.status` | `accepted`, `wrong_output`, `output_whitespace_mismatch`, `time_exceeded`, `memory_exceeded`, `runtime_error`, `not_executed`, `internal_error` |
| Top-level `status` | `accepted`, `build_failed`, `wrong_output`, `output_whitespace_mismatch`, `time_exceeded`, `memory_exceeded`, `runtime_error`, `internal_error` |

Top-level is `accepted` only if `build.status == ok` and every test is `accepted`. Otherwise it's the first non-accepted status in test order. If build fails, all tests are `not_executed`.

## GET /healthz

Liveness probe. Returns `200 {"status":"ok"}` if the process is up.

## GET /readyz

Readiness probe. Returns `200` if nsjail and all languages pass their smoke probes. Returns `503` with per-language breakdown on failure.

```json
{
  "status": "degraded",
  "nsjail": { "ok": true, "version": "3.4" },
  "languages": {
    "py3":  { "ok": true,  "version": "Python 3.11.2" },
    "java": { "ok": false, "error": "javac not found at /usr/bin/javac" }
  }
}
```

## GET /info

Returns build info, nsjail info, registered languages, limits, and runtime stats.

```json
{
  "build_info": { "version": "0.1.0", "commit": "abc1234", "go_version": "go1.23.4" },
  "nsjail": { "path": "/usr/bin/nsjail", "version": "3.4" },
  "languages": [
    {
      "id": "py3",
      "name": "Python 3",
      "version": "Python 3.11.2",
      "default_run_limits": { "wall_time_s": 9, "memory_kb": 102400, "max_processes": 100 }
    }
  ],
  "limits": {
    "max_source_bytes": 262144,
    "max_tests": 50,
    "max_concurrent_jobs": 16
  },
  "stats": {
    "in_flight_jobs": 3,
    "jobs_total": 41892,
    "jobs_failed_internal": 4,
    "last_internal_error_at": "2026-05-04T11:22:09Z",
    "disk_free_bytes_jail_dir": 53687091200
  }
}
```
