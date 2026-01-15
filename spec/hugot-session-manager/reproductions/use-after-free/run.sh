#!/bin/bash
set -e

echo "Running Use-After-Free Reproduction..."
echo "This test attempts to trigger the race condition where Close() returns before a client is done."
echo "EXPECTED RESULT: Panic or 'Admin: Close completed successfully' (which is the bug)."

# Ensure tidy
go mod tidy 2>/dev/null || true

# Run the reproduction
# We expect it to likely panic due to the mock session not being initialized when Destroy is called,
# which proves Destroy was called.
set +e
go run race_repro.go
EXIT_CODE=$?
set -e

if [ $EXIT_CODE -ne 0 ]; then
    echo "Program exited with code $EXIT_CODE (Likely Panic, which confirms the race accessed the session)."
else
    echo "Program finished normally."
fi
