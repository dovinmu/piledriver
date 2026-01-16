# pilemonkey

A real-time file diff viewer TUI for watching agent-driven code changes.

## Overview

When working with AI coding agents (like Claude Code), it's helpful to see what changes are being made in real-time. `pilemonkey` watches a directory for file changes and displays diffs with syntax highlighting, allowing you to navigate through the change history.

## Features

- Real-time file watching with debouncing
- Diff highlighting (green for additions, red for removals)
- Navigate through change history with arrow keys
- Live mode auto-follows the latest changes
- Vim-style key bindings
- Ignores common directories (.git, node_modules, etc.)

## Installation

### From source

```bash
cd pilemonkey
make build
sudo make install-global  # installs to /usr/local/bin
```

### Go install

```bash
go install github.com/dovinmu/piledriver/pilemonkey@latest
```

## Usage

```bash
# Watch current directory
pilemonkey

# Watch a specific directory
pilemonkey ~/projects/myapp

# Custom history size
pilemonkey -n 50

# Additional ignore patterns
pilemonkey -i "*.log,tmp"
```

## Key Bindings

| Key | Action |
|-----|--------|
| `←` / `h` | Previous changeset |
| `→` / `l` | Next changeset |
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| `PgUp` / `Ctrl+b` | Page up |
| `PgDn` / `Ctrl+f` | Page down |
| `g` / `Home` | Top of diff |
| `G` / `End` | Bottom of diff |
| `L` | Return to live mode |
| `q` / `Ctrl+c` | Quit |

## Live Mode

- **Enabled by default**: New changes automatically appear
- **Disabled when navigating back**: Press `←` to go to previous changes
- **Re-enabled when reaching newest**: Navigate forward to the latest change
- **Manual return**: Press `L` to jump to live mode

## Requirements

- Go 1.21+
- Git (for diff computation)

## License

MIT
