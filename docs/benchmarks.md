# Benchmarks

Load test results for goboxd.

## Setup

- **Workload**: Python 3 "Hello World" — `print("hello")` with one test case
- **Tool**: [hey](https://github.com/rakyll/hey)
- **Container**: Docker on Linux (amd64)
- **Concurrency limit**: `runtime.NumCPU()` (default)

## Results

_Run `make load` against a clean Docker container to fill in these results._

### 1 Concurrent Client

| Metric | Value |
|--------|-------|
| Requests/sec | _TBD_ |
| p50 latency | _TBD_ |
| p95 latency | _TBD_ |
| p99 latency | _TBD_ |

### 10 Concurrent Clients

| Metric | Value |
|--------|-------|
| Requests/sec | _TBD_ |
| p50 latency | _TBD_ |
| p95 latency | _TBD_ |
| p99 latency | _TBD_ |

### 50 Concurrent Clients

| Metric | Value |
|--------|-------|
| Requests/sec | _TBD_ |
| p50 latency | _TBD_ |
| p95 latency | _TBD_ |
| p99 latency | _TBD_ |

### 100 Concurrent Clients

| Metric | Value |
|--------|-------|
| Requests/sec | _TBD_ |
| p50 latency | _TBD_ |
| p95 latency | _TBD_ |
| p99 latency | _TBD_ |

## Methodology

```bash
# Start the server
make build && make run-detached

# Run load test
make load

# Stop
make down
```

The load test script runs `hey` at 1, 10, 50, and 100 concurrent clients against `POST /run` with a Python 3 hello world payload. Each concurrency level sends `concurrency × 20` total requests.

## Notes

- Results are from a clean Docker run, not from a debugger
- The worker pool uses a bounded semaphore — when all slots are taken, requests queue rather than fail
- Per-request CPU and wall-clock time are captured in structured JSON logs
