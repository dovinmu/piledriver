#!/bin/bash
set -e

# Configuration
TERMITE_DIR="../../../../termite"
# Point to user's home models dir as requested
MODELS_DIR="$HOME/.termite/models"
MODEL_NAME="BAAI/bge-small-en-v1.5"
API_URL="http://localhost:8080/api/embed"

echo "=== Termite E2E Verification ==="
echo "Building Termite..."
cd "$TERMITE_DIR"
make build-termite
cd - > /dev/null

TERMITE_BIN="$TERMITE_DIR/bin/termite"

echo "Starting Termite Server..."
export TERMITE_MODELS_DIR="$MODELS_DIR"
export TERMITE_API_URL="http://0.0.0.0:8080"
# Run in background
$TERMITE_BIN run &
SERVER_PID=$!

cleanup() {
    echo "Stopping Termite (PID $SERVER_PID)..."
    kill $SERVER_PID || true
    wait $SERVER_PID || true
}
trap cleanup EXIT

echo "Waiting for server to start..."
sleep 5

echo "Test 1: Single Batch Request"
RESPONSE=$(curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"$MODEL_NAME\",
    \"input\": [\"Hello world\"]
  }")

# Check if response contains embeddings
if echo "$RESPONSE" | grep -q "embeddings"; then
    echo "PASS: Single batch received embeddings."
else
    echo "FAIL: Single batch response invalid."
    echo "$RESPONSE"
    exit 1
fi

echo "Test 2: Multiple Batches (simulate load)"
for i in {1..5}; do
    echo "Sending batch $i..."
    curl -s -X POST "$API_URL" \
      -H "Content-Type: application/json" \
      -d "{
        \"model\": \"$MODEL_NAME\",
        \"input\": [\"Batch $i item 1\", \"Batch $i item 2\"]
      }" > /dev/null
done
echo "PASS: Multiple batches sent."

echo "=== E2E Verification Complete ==="
