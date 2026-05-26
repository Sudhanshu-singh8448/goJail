#!/bin/bash
# Load test for goboxd using hey (https://github.com/rakyll/hey).
# Usage: ./tests/load_test.sh [concurrency]

set -e

BASE_URL="${GOBOXD_URL:-http://localhost:8000}"
RESULTS_DIR="docs"

# Check if hey is installed
if ! command -v hey &> /dev/null; then
    echo "hey is not installed. Install with: go install github.com/rakyll/hey@latest"
    echo "Or on macOS: brew install hey"
    exit 1
fi

echo "=== goboxd Load Test ==="
echo "Target: $BASE_URL"
echo ""

# Create a test payload
PAYLOAD='{"language":"py3","source":"print(\"hello\")","tests":[{"stdin":"","expected_stdout":"hello\n"}]}'

run_bench() {
    local concurrency=$1
    local requests=$((concurrency * 20))

    echo "--- $concurrency concurrent clients, $requests total requests ---"
    hey -n "$requests" -c "$concurrency" -m POST \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD" \
        "$BASE_URL/run" 2>&1 | grep -E "Requests/sec|Latency distribution|50%|75%|90%|95%|99%|Total:"
    echo ""
}

echo "Warming up..."
curl -s -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" > /dev/null

echo ""
run_bench 1
run_bench 10
run_bench 50
run_bench 100

echo "=== Load test complete ==="
echo "Copy the results above into $RESULTS_DIR/benchmarks.md"
