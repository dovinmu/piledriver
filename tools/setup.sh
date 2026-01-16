#!/bin/bash
set -e

# Downloads tla2tools.jar if not present

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JAR_PATH="$SCRIPT_DIR/tla2tools.jar"

# TLA+ tools release - update version as needed
TLA_VERSION="1.8.0"
TLA_URL="https://github.com/tlaplus/tlaplus/releases/download/v${TLA_VERSION}/tla2tools.jar"

if [ -f "$JAR_PATH" ]; then
    echo "tla2tools.jar already exists at $JAR_PATH"
    exit 0
fi

echo "Downloading tla2tools.jar v${TLA_VERSION}..."
curl -L -o "$JAR_PATH" "$TLA_URL"

echo "Downloaded to $JAR_PATH"
