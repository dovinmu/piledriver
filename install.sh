#!/usr/bin/env bash
set -e

# Piledriver installer
# Downloads TLA+ tools, optionally installs Claude Code slash command

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAUDE_DIR="$HOME/.claude"
COMMANDS_DIR="$CLAUDE_DIR/commands"

echo "Piledriver Installer"
echo "===================="
echo

# Check for required files
if [ ! -f "$SCRIPT_DIR/piledriver" ]; then
    echo "Error: piledriver executable not found in $SCRIPT_DIR"
    exit 1
fi

# 1. Download TLA+ tools
echo "Checking TLA+ tools..."
"$SCRIPT_DIR/tools/setup.sh"
echo

# 2. PATH guidance
if ! command -v piledriver &>/dev/null; then
    echo "To use piledriver from anywhere, add it to your PATH:"
    echo "  export PATH=\"$SCRIPT_DIR:\$PATH\""
    echo
    echo "Or create a symlink:"
    echo "  ln -s $SCRIPT_DIR/piledriver /usr/local/bin/piledriver"
    echo
fi

# 3. Claude Code slash command (optional)
if [ -d "$CLAUDE_DIR" ]; then
    mkdir -p "$COMMANDS_DIR"

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

    echo "Installed Claude Code slash command: /piledriver"
    echo "  Usage: /piledriver I think there's a race condition in the session manager"
    echo
else
    echo "Claude Code not detected (~/.claude not found) — skipping slash command."
    echo "  If you install Claude Code later, re-run this script to add the /piledriver command."
    echo
fi

# 4. Guidance for other agents
echo "For use with other AI agents, point them at AGENTS.md:"
echo "  $SCRIPT_DIR/AGENTS.md"
echo
echo "Done."
