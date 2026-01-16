#!/bin/bash
set -e

# Piledriver installer for Claude Code
# Installs piledriver as a Claude Code skill

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAUDE_DIR="$HOME/.claude"
SKILLS_DIR="$CLAUDE_DIR/skills"
PILEDRIVER_SKILL_DIR="$SKILLS_DIR/piledriver"

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

# Create skills directory if needed
mkdir -p "$PILEDRIVER_SKILL_DIR"

# Generate SKILL.md with the full path to piledriver
cat > "$PILEDRIVER_SKILL_DIR/SKILL.md" << EOF
# Piledriver Skill

Use this skill when the user wants to do systematic bug hunting using TLA+ formal methods.

Trigger: User says "/piledriver" or asks to hunt for bugs using formal methods.

## Piledriver Executable

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

## Important

The \`.piledriver/\` directory is created in the **current working directory** where Claude is running, NOT in the piledriver source directory. This is the correct behavior - each project gets its own .piledriver/ for its bug hunts.

---

EOF

# Append the full CLAUDE.md content
cat "$SCRIPT_DIR/CLAUDE.md" >> "$PILEDRIVER_SKILL_DIR/SKILL.md"

echo "Installed piledriver skill to: $PILEDRIVER_SKILL_DIR"
echo
echo "Files created:"
echo "  - $PILEDRIVER_SKILL_DIR/SKILL.md"
echo
echo "Usage:"
echo "  From any directory, run 'claude' and use:"
echo "    /piledriver <starting-point>"
echo
echo "  Example:"
echo "    /piledriver I think there's a race condition in the session manager"
echo
echo "Done."
