#!/bin/bash
# Integration test suite for goboxd.
# Requires Docker container running on localhost:8000.

set -e

BASE_URL="${GOBOXD_URL:-http://localhost:8000}"
PASS=0
FAIL=0
TOTAL=0

green() { echo -e "\033[32m$1\033[0m"; }
red()   { echo -e "\033[31m$1\033[0m"; }
bold()  { echo -e "\033[1m$1\033[0m"; }

assert_status() {
    local name="$1" expected_http="$2" expected_status="$3" response="$4" http_code="$5"
    TOTAL=$((TOTAL + 1))

    if [ "$http_code" != "$expected_http" ]; then
        red "FAIL [$name]: expected HTTP $expected_http, got $http_code"
        echo "  Response: $response"
        FAIL=$((FAIL + 1))
        return
    fi

    if [ -n "$expected_status" ]; then
        actual_status=$(echo "$response" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null || echo "")
        if [ "$actual_status" != "$expected_status" ]; then
            red "FAIL [$name]: expected status '$expected_status', got '$actual_status'"
            echo "  Response: $response"
            FAIL=$((FAIL + 1))
            return
        fi
    fi

    green "PASS [$name]"
    PASS=$((PASS + 1))
}

bold "=== goboxd Integration Tests ==="
echo "Target: $BASE_URL"
echo ""

# --- Health endpoints ---
bold "--- Health Endpoints ---"

RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/healthz")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "GET /healthz" "200" "ok" "$BODY" "$HTTP"

RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/readyz")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
# readyz may be 200 or 503, just check it responds
TOTAL=$((TOTAL + 1))
if [ "$HTTP" = "200" ] || [ "$HTTP" = "503" ]; then
    green "PASS [GET /readyz responds]"
    PASS=$((PASS + 1))
else
    red "FAIL [GET /readyz responds]: got HTTP $HTTP"
    FAIL=$((FAIL + 1))
fi

RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/info")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
TOTAL=$((TOTAL + 1))
if [ "$HTTP" = "200" ]; then
    green "PASS [GET /info]"
    PASS=$((PASS + 1))
else
    red "FAIL [GET /info]: got HTTP $HTTP"
    FAIL=$((FAIL + 1))
fi

# --- Language tests ---
bold ""
bold "--- Language Tests ---"

# Python 3
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"py3","source":"print(\"hello\")","tests":[{"stdin":"","expected_stdout":"hello\n"}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run py3 hello" "200" "accepted" "$BODY" "$HTTP"

# C++
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"hi\";return 0;}","tests":[{"stdin":"","expected_stdout":"hi"}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run cpp hello" "200" "accepted" "$BODY" "$HTTP"

# C
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"c","source":"#include <stdio.h>\nint main(){printf(\"hi\");return 0;}","tests":[{"stdin":"","expected_stdout":"hi"}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run c hello" "200" "accepted" "$BODY" "$HTTP"

# Bash
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"bash","source":"echo hello","tests":[{"stdin":"","expected_stdout":"hello\n"}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run bash hello" "200" "accepted" "$BODY" "$HTTP"

# JavaScript
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"javascript","source":"console.log(\"hello\")","tests":[{"stdin":"","expected_stdout":"hello\n"}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run javascript hello" "200" "accepted" "$BODY" "$HTTP"

# Java
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"java","source":"public class Main { public static void main(String[] args) { System.out.print(\"hi\"); } }","source_filename":"Main.java","artifact_filename":"Main","tests":[{"stdin":"","expected_stdout":"hi"}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run java hello" "200" "accepted" "$BODY" "$HTTP"

# --- Error case tests ---
bold ""
bold "--- Error Cases ---"

# Unknown language
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"rust","source":"fn main(){}","tests":[{"stdin":"","expected_stdout":""}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run unknown language" "400" "" "$BODY" "$HTTP"

# Missing source
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"py3","tests":[{"stdin":"","expected_stdout":""}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run missing source" "400" "" "$BODY" "$HTTP"

# Path traversal filename
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"py3","source":"print(1)","source_filename":"../../etc/passwd","tests":[{"stdin":"","expected_stdout":"1\n"}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run path traversal filename" "400" "" "$BODY" "$HTTP"

# Disallowed flag
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d '{"language":"cpp","source":"int main(){}","build":{"flags":["-fplugin=evil.so"]},"tests":[{"stdin":"","expected_stdout":""}]}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
assert_status "POST /run disallowed flag" "400" "" "$BODY" "$HTTP"

# Invalid JSON
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d 'not json')
HTTP=$(echo "$RESP" | tail -1)
assert_status "POST /run invalid JSON" "400" "" "" "$HTTP"

# --- Summary ---
bold ""
bold "=== Results: $PASS/$TOTAL passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
