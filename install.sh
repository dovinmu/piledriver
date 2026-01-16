#!/bin/bash
set -e

# Piledriver installer for Claude Code
# Installs piledriver as a Claude Code command

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAUDE_DIR="$HOME/.claude"
COMMANDS_DIR="$CLAUDE_DIR/commands"

echo "Piledriver Installer"
echo "===================="
echo

# Check for .claude directory
if [ ! -d "$CLAUDE_DIR" ]; then
    echo "Error: $CLAUDE_DIR not found."
    echo "This installer requires Claude Code to be installed."
    echo "Install Claude Code first, then run this installer again."
    exit 1
fi

# Check for required files
if [ ! -f "$SCRIPT_DIR/piledriver" ]; then
    echo "Error: piledriver executable not found in $SCRIPT_DIR"
    exit 1
fi

if [ ! -f "$SCRIPT_DIR/CLAUDE.md" ]; then
    echo "Error: CLAUDE.md not found in $SCRIPT_DIR"
    exit 1
fi

# Download TLA+ tools if needed
echo "Checking TLA+ tools..."
"$SCRIPT_DIR/tools/setup.sh"
echo

# Create commands directory if needed
mkdir -p "$COMMANDS_DIR"

# Generate piledriver.md command file
cat > "$COMMANDS_DIR/piledriver.md" << EOF
---
description: Systematic bug hunting using TLA+ formal methods
argument-hint: <suspect-description>
---

# Piledriver Bug Hunt

Start a systematic bug hunt using TLA+ formal methods.

## Piledriver CLI

The piledriver CLI is located at:
\`\`\`
$SCRIPT_DIR/piledriver
\`\`\`

Run commands like:
\`\`\`bash
$SCRIPT_DIR/piledriver init <session-name>
$SCRIPT_DIR/piledriver bug <session-name> <bug-name>
$SCRIPT_DIR/piledriver test <session-name> [bug-name]
$SCRIPT_DIR/piledriver status [session-name]
\`\`\`

## Starting the Hunt

The user wants to investigate: \$ARGUMENTS

Begin by initializing a hunt session and entering SCOPING mode.

## Important

The \`.piledriver/\` directory is created in the **current working directory** where Claude is running, NOT in the piledriver source directory. This is the correct behavior - each project gets its own .piledriver/ for its bug hunts.

---

EOF

# Append the full CLAUDE.md content
cat "$SCRIPT_DIR/CLAUDE.md" >> "$COMMANDS_DIR/piledriver.md"

echo "Installed piledriver command to: $COMMANDS_DIR/piledriver.md"
echo
echo "Usage:"
echo "  From any directory, run 'claude' and use:"
echo "    /piledriver <starting-point>"
echo
echo "  Example:"
echo "    /piledriver I think there's a race condition in the session manager"
echo
echo "Done."
