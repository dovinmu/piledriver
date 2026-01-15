#!/usr/bin/env bash
# E2E Happy Path Test for PooledHugotEmbedder
#
# This script builds termite, starts the server, loads a model,
# sends embedding requests, and verifies everything works.
#
# Usage:
#   cd termite
#   ../spec/hugot-usage/reproducers/e2e_happy_path.sh
#
# Prerequisites:
#   - ONNX Runtime downloaded (make e2e-deps)
#   - Model available at ~/.termite/models/embedders/BAAI/bge-small-en-v1.5

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Configuration
PORT=${PORT:-11444}
HOST="http://localhost:${PORT}"
MODEL="BAAI/bge-small-en-v1.5"
TERMITE_PID=""
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cleanup() {
    if [[ -n "$TERMITE_PID" ]] && kill -0 "$TERMITE_PID" 2>/dev/null; then
        log_info "Stopping termite server (PID: $TERMITE_PID)..."
        kill "$TERMITE_PID" 2>/dev/null || true
        wait "$TERMITE_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Detect platform
UNAME_S=$(uname -s)
UNAME_M=$(uname -m)
if [[ "$UNAME_S" == "Darwin" ]]; then
    if [[ "$UNAME_M" == "arm64" ]]; then
        PLATFORM="darwin-arm64"
    else
        PLATFORM="darwin-amd64"
    fi
else
    if [[ "$UNAME_M" == "aarch64" ]]; then
        PLATFORM="linux-arm64"
    else
        PLATFORM="linux-amd64"
    fi
fi

# Check we're in termite directory (workspace with go.work or module with go.mod)
if [[ -f "go.work" ]]; then
    # Workspace mode - check for pkg/termite
    if [[ ! -d "pkg/termite" ]]; then
        log_error "Please run this script from the termite directory"
        exit 1
    fi
elif [[ -f "go.mod" ]]; then
    # Module mode
    if ! grep -q "github.com/antflydb/termite" go.mod 2>/dev/null; then
        log_error "Please run this script from the termite directory"
        exit 1
    fi
else
    log_error "Please run this script from the termite directory"
    exit 1
fi

# Set up environment
export ONNXRUNTIME_ROOT="${ONNXRUNTIME_ROOT:-$PWD/onnxruntime}"
export CGO_ENABLED=1
export LIBRARY_PATH="${ONNXRUNTIME_ROOT}/${PLATFORM}/lib:${LIBRARY_PATH:-}"
export LD_LIBRARY_PATH="${ONNXRUNTIME_ROOT}/${PLATFORM}/lib:${LD_LIBRARY_PATH:-}"
export DYLD_LIBRARY_PATH="${ONNXRUNTIME_ROOT}/${PLATFORM}/lib:${DYLD_LIBRARY_PATH:-}"

log_info "Platform: $PLATFORM"
log_info "ONNXRUNTIME_ROOT: $ONNXRUNTIME_ROOT"

# Check ONNX Runtime exists
if [[ ! -d "$ONNXRUNTIME_ROOT/$PLATFORM/lib" ]]; then
    log_error "ONNX Runtime not found. Run 'make e2e-deps' first."
    exit 1
fi

# Step 1: Build termite
log_info "Building termite with ONNX backend..."
go build -tags="onnx,ORT" -o ./termite-test ./pkg/termite/cmd

if [[ ! -x "./termite-test" ]]; then
    log_error "Build failed"
    exit 1
fi
log_info "Build successful"

# Step 2: Start server
log_info "Starting termite server on port $PORT..."
# Port is configured via TERMITE_API_URL env var or api_url in config
export TERMITE_API_URL="http://localhost:${PORT}"
./termite-test run --log-level info &
TERMITE_PID=$!

# Wait for server to be ready
log_info "Waiting for server to be ready..."
MAX_WAIT=30
for i in $(seq 1 $MAX_WAIT); do
    if curl -s "${HOST}/healthz" > /dev/null 2>&1; then
        log_info "Server ready after ${i}s"
        break
    fi
    if ! kill -0 "$TERMITE_PID" 2>/dev/null; then
        log_error "Server process died"
        exit 1
    fi
    sleep 1
done

if ! curl -s "${HOST}/healthz" > /dev/null 2>&1; then
    log_error "Server failed to start within ${MAX_WAIT}s"
    exit 1
fi

# Step 3: Test health endpoint
log_info "Testing health endpoint..."
HEALTH=$(curl -s "${HOST}/healthz")
echo "  Response: $HEALTH"

# Step 4: Load model and generate embeddings
log_info "Testing embeddings endpoint with model: $MODEL"

# Single text
log_info "Test 1: Single text embedding..."
RESPONSE=$(curl -s -X POST "${HOST}/api/embed" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    -d "{
        \"model\": \"${MODEL}\",
        \"input\": \"Hello, world!\"
    }")

if echo "$RESPONSE" | grep -q '"embeddings"'; then
    DIM=$(echo "$RESPONSE" | grep -o '"embeddings":\[\[[^]]*\]' | tr ',' '\n' | wc -l)
    log_info "  SUCCESS: Got embedding with ~${DIM} dimensions"
else
    log_error "  FAILED: $RESPONSE"
    exit 1
fi

# Multiple texts
log_info "Test 2: Multiple text embeddings..."
RESPONSE=$(curl -s -X POST "${HOST}/api/embed" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    -d "{
        \"model\": \"${MODEL}\",
        \"input\": [
            \"First sentence\",
            \"Second sentence\",
            \"Third sentence\"
        ]
    }")

# Count embeddings arrays in response (termite returns {"embeddings": [[...], [...], [...]]})
if echo "$RESPONSE" | grep -q '"embeddings"'; then
    # Count opening brackets after "embeddings":[ to count number of embedding vectors
    COUNT=$(echo "$RESPONSE" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('embeddings',[])))" 2>/dev/null || echo "0")
    if [[ "$COUNT" -eq 3 ]]; then
        log_info "  SUCCESS: Got 3 embeddings"
    else
        log_error "  FAILED: Expected 3 embeddings, got $COUNT"
        log_error "  Response: $RESPONSE"
        exit 1
    fi
else
    log_error "  FAILED: No embeddings in response"
    log_error "  Response: $RESPONSE"
    exit 1
fi

# Concurrent requests
log_info "Test 3: Concurrent requests (10 parallel)..."
TEMP_DIR=$(mktemp -d)
PIDS=""
for i in $(seq 1 10); do
    curl -s -X POST "${HOST}/api/embed" \
        -H "Content-Type: application/json" \
    -H "Accept: application/json" \
        -d "{
            \"model\": \"${MODEL}\",
            \"input\": \"Concurrent test $i\"
        }" > "${TEMP_DIR}/response_${i}.json" &
    PIDS="$PIDS $!"
done

# Wait for all requests
FAILED=0
for pid in $PIDS; do
    if ! wait "$pid"; then
        FAILED=$((FAILED + 1))
    fi
done

# Check responses
SUCCESS=0
for i in $(seq 1 10); do
    if grep -q '"embeddings"' "${TEMP_DIR}/response_${i}.json" 2>/dev/null; then
        SUCCESS=$((SUCCESS + 1))
    fi
done
rm -rf "$TEMP_DIR"

if [[ "$SUCCESS" -eq 10 ]]; then
    log_info "  SUCCESS: All 10 concurrent requests succeeded"
else
    log_error "  FAILED: Only $SUCCESS/10 requests succeeded"
    exit 1
fi

# Large batch
log_info "Test 4: Large batch (20 texts)..."
TEXTS=""
for i in $(seq 1 20); do
    if [[ -n "$TEXTS" ]]; then
        TEXTS="${TEXTS},"
    fi
    TEXTS="${TEXTS}\"Batch test sentence number $i with some extra words\""
done

RESPONSE=$(curl -s -X POST "${HOST}/api/embed" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    -d "{
        \"model\": \"${MODEL}\",
        \"input\": [${TEXTS}]
    }")

if echo "$RESPONSE" | grep -q '"embeddings"'; then
    COUNT=$(echo "$RESPONSE" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('embeddings',[])))" 2>/dev/null || echo "0")
    if [[ "$COUNT" -eq 20 ]]; then
        log_info "  SUCCESS: Got 20 embeddings"
    else
        log_error "  FAILED: Expected 20 embeddings, got $COUNT"
        log_error "  Response (truncated): ${RESPONSE:0:500}..."
        exit 1
    fi
else
    log_error "  FAILED: No embeddings in response"
    log_error "  Response: $RESPONSE"
    exit 1
fi

# Step 5: Graceful shutdown
log_info "Test 5: Graceful shutdown..."
kill "$TERMITE_PID"
WAIT_START=$(date +%s)
while kill -0 "$TERMITE_PID" 2>/dev/null; do
    NOW=$(date +%s)
    if [[ $((NOW - WAIT_START)) -gt 10 ]]; then
        log_error "  FAILED: Server didn't shut down within 10s"
        kill -9 "$TERMITE_PID" 2>/dev/null || true
        exit 1
    fi
    sleep 0.5
done
TERMITE_PID=""
log_info "  SUCCESS: Server shut down gracefully"

# Summary
echo ""
echo "========================================"
log_info "E2E Happy Path: ALL TESTS PASSED"
echo "========================================"
echo ""
echo "Tests completed:"
echo "  1. Single text embedding     ✓"
echo "  2. Multiple text embeddings  ✓"
echo "  3. Concurrent requests (10)  ✓"
echo "  4. Large batch (20 texts)    ✓"
echo "  5. Graceful shutdown         ✓"
echo ""

# Cleanup build artifact
rm -f ./termite-test
