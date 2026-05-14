#!/usr/bin/env bash
set -e

# Piledriver installer
# Downloads TLA+ tools, installs agent integrations

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAUDE_DIR="$HOME/.claude"
COMMANDS_DIR="$CLAUDE_DIR/commands"
CODEX_DIR="$HOME/.codex"
CODEX_SKILLS_DIR="$CODEX_DIR/skills"
BIN_DIR="${PILEDRIVER_INSTALL_BIN:-$HOME/.local/bin}"

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

# 2. Install CLI on PATH
echo "Installing piledriver CLI..."
mkdir -p "$BIN_DIR"
BIN_PATH="$BIN_DIR/piledriver"

if [ "$BIN_PATH" -ef "$SCRIPT_DIR/piledriver" ] 2>/dev/null; then
    echo "[OK] piledriver already linked at $BIN_PATH"
elif [ -L "$BIN_PATH" ]; then
    ln -sfn "$SCRIPT_DIR/piledriver" "$BIN_PATH"
    echo "[OK] Updated symlink: $BIN_PATH -> $SCRIPT_DIR/piledriver"
elif [ -e "$BIN_PATH" ]; then
    echo "[WARN] $BIN_PATH already exists and is not a symlink; leaving it unchanged."
    echo "       To install this copy, remove that file or set PILEDRIVER_INSTALL_BIN to another directory."
else
    ln -s "$SCRIPT_DIR/piledriver" "$BIN_PATH"
    echo "[OK] Created symlink: $BIN_PATH -> $SCRIPT_DIR/piledriver"
fi

case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *)
        echo "[WARN] $BIN_DIR is not on PATH."
        echo "       Add this to your shell profile:"
        echo "         export PATH=\"$BIN_DIR:\$PATH\""
        ;;
esac

if command -v piledriver &>/dev/null; then
    RESOLVED_PILEDRIVER="$(command -v piledriver)"
    if [ "$RESOLVED_PILEDRIVER" != "$BIN_PATH" ] && [ "$RESOLVED_PILEDRIVER" != "$SCRIPT_DIR/piledriver" ]; then
        echo "[WARN] 'piledriver' currently resolves to $RESOLVED_PILEDRIVER before $BIN_PATH on PATH."
        echo "       Reorder PATH if you want this checkout to take precedence."
    else
        echo "[OK] 'piledriver' is runnable from this shell."
    fi
elif [ -x "$BIN_PATH" ]; then
    echo "[WARN] Symlink installed, but 'piledriver' is not visible in this shell's PATH yet."
else
    echo "[WARN] piledriver was not installed on PATH."
fi
echo

# 3. Codex skill
echo "Installing Codex skill..."
mkdir -p "$CODEX_SKILLS_DIR/piledriver"

cat > "$CODEX_SKILLS_DIR/piledriver/SKILL.md" << EOF
---
name: piledriver
description: Use when the user wants to start, run, or continue a Piledriver bug-hunting workflow; investigate a suspected bug with formal methods; define verification boundaries and assumptions; create reproducers; run piledriver CLI commands; or follow Piledriver's red/green testing and report process.
metadata:
  short-description: Systematic formal-methods bug hunting
---

EOF

cat "$SCRIPT_DIR/AGENTS.md" >> "$CODEX_SKILLS_DIR/piledriver/SKILL.md"

echo "Installed Codex skill: piledriver"
echo "  Usage: ask Codex to \"use the piledriver skill\" when starting a bug hunt."
echo

# 4. Claude Code slash command (optional)
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

# 5. Guidance for other agents
echo "For use with other AI agents, point them at AGENTS.md:"
echo "  $SCRIPT_DIR/AGENTS.md"
echo
echo "Done."
