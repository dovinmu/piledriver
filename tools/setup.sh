#!/bin/bash
set -e

# Piledriver setup: checks dependencies and downloads tools

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JAR_PATH="$SCRIPT_DIR/tla2tools.jar"

# TLA+ tools release - update version as needed
TLA_VERSION="1.8.0"
TLA_URL="https://github.com/tlaplus/tlaplus/releases/download/v${TLA_VERSION}/tla2tools.jar"

echo "Piledriver Setup"
echo "================"
echo

# Check for Java
check_java() {
    if command -v java &> /dev/null && java -version 2>&1 | grep -q "version"; then
        JAVA_VERSION=$(java -version 2>&1 | head -1)
        echo "[OK] Java found: $JAVA_VERSION"
        return 0
    else
        echo "[MISSING] Java not found or not working"
        echo
        echo "Install Java (JRE 11+) using one of:"
        echo "  macOS:  brew install openjdk@17"
        echo "  Ubuntu: sudo apt install openjdk-17-jre"
        echo "  Fedora: sudo dnf install java-17-openjdk"
        echo
        return 1
    fi
}

# Download tla2tools.jar
download_tla2tools() {
    if [ -f "$JAR_PATH" ]; then
        echo "[OK] tla2tools.jar exists at $JAR_PATH"
        return 0
    fi

    echo "Downloading tla2tools.jar v${TLA_VERSION}..."
    if curl -L -o "$JAR_PATH" "$TLA_URL"; then
        echo "[OK] Downloaded tla2tools.jar"
        return 0
    else
        echo "[ERROR] Failed to download tla2tools.jar"
        return 1
    fi
}

# Run checks
MISSING=0

check_java || MISSING=1
download_tla2tools || MISSING=1

echo
if [ $MISSING -eq 0 ]; then
    echo "Setup complete! Run 'piledriver check <session>' to verify TLA+ specs."
else
    echo "Some dependencies are missing. Install them and re-run this script."
    exit 1
fi
