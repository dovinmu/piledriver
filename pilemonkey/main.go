package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dovinmu/piledriver/pilemonkey/state"
	"github.com/dovinmu/piledriver/pilemonkey/store"
	"github.com/dovinmu/piledriver/pilemonkey/summarizer"
	"github.com/dovinmu/piledriver/pilemonkey/tui"
	"github.com/dovinmu/piledriver/pilemonkey/watcher"
)

var (
	version = "0.3.2"
)

func main() {
	// Parse flags
	historySize := flag.Int("n", 100, "Number of changesets to keep in history")
	ignorePatterns := flag.String("i", "", "Additional patterns to ignore (comma-separated)")
	sessionName := flag.String("s", "", "Piledriver session name (auto-detected if not specified)")
	showVersion := flag.Bool("v", false, "Show version")
	enableSummarizer := flag.Bool("summarizer", true, "Enable conversation summarizer (Claude Code and opencode)")
	summarizerSource := flag.String("summarizer-source", "auto", "Conversation source: auto, claude, or opencode")
	summarizerDebug := flag.Bool("summarizer-debug", false, "Enable debug logging for summarizer")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "pilemonkey - Real-time file diff viewer for watching agent changes\n\n")
		fmt.Fprintf(os.Stderr, "Usage: pilemonkey [flags] [directory]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDiff Mode Keys:\n")
		fmt.Fprintf(os.Stderr, "  Tab       Toggle overview mode\n")
		fmt.Fprintf(os.Stderr, "  ←/h       Previous changeset\n")
		fmt.Fprintf(os.Stderr, "  →/l       Next changeset\n")
		fmt.Fprintf(os.Stderr, "  ↑/k       Scroll up\n")
		fmt.Fprintf(os.Stderr, "  ↓/j       Scroll down\n")
		fmt.Fprintf(os.Stderr, "  PgUp      Page up\n")
		fmt.Fprintf(os.Stderr, "  PgDn      Page down\n")
		fmt.Fprintf(os.Stderr, "  g/Home    Top of diff\n")
		fmt.Fprintf(os.Stderr, "  G/End     Bottom of diff\n")
		fmt.Fprintf(os.Stderr, "  L         Return to live mode\n")
		fmt.Fprintf(os.Stderr, "  q         Quit\n")
		fmt.Fprintf(os.Stderr, "\nOverview Mode Keys:\n")
		fmt.Fprintf(os.Stderr, "  Tab       Toggle diff mode\n")
		fmt.Fprintf(os.Stderr, "  ←/h       Previous phase\n")
		fmt.Fprintf(os.Stderr, "  →/l       Next phase\n")
		fmt.Fprintf(os.Stderr, "  n         Add/edit note for phase\n")
		fmt.Fprintf(os.Stderr, "  q         Quit\n")
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("pilemonkey v%s\n", version)
		os.Exit(0)
	}

	// Get directory to watch
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	// Resolve to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	// Verify directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", absDir)
		os.Exit(1)
	}

	// Parse ignore patterns
	var extraIgnore []string
	if *ignorePatterns != "" {
		extraIgnore = strings.Split(*ignorePatterns, ",")
		for i := range extraIgnore {
			extraIgnore[i] = strings.TrimSpace(extraIgnore[i])
		}
	}

	// Create store
	s := store.New(*historySize)

	// Create watcher
	w, err := watcher.New(absDir, s, extraIgnore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating watcher: %v\n", err)
		os.Exit(1)
	}

	// Create TUI model
	model := tui.NewModel(s, absDir)

	// Try to find and load piledriver session
	var sessionInfo *state.SessionInfo
	var activeSession string
	piledriverDir, _ := state.FindPiledriverDir(absDir)
	if piledriverDir != "" {
		if *sessionName != "" {
			activeSession = *sessionName
		} else {
			// Auto-detect most recent session
			activeSession, _ = state.FindMostRecentSession(piledriverDir)
		}

		if activeSession != "" {
			info, err := state.GetSessionInfo(piledriverDir, activeSession)
			if err == nil {
				sessionInfo = info
				model.SetSessionInfo(sessionInfo)
			}
		}
	}

	// Create bubbletea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Set up watcher callback to send messages to TUI
	w.OnChangeset = func(cs store.Changeset) {
		p.Send(tui.ChangesetMsg{Changeset: cs})
	}

	// Start conversation summarizer if enabled
	if *enableSummarizer {
		sum, err := summarizer.New(absDir, summarizer.Options{
			SourcePreference: *summarizerSource,
			Debug:            *summarizerDebug,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not start summarizer: %v\n", err)
		} else {
			sum.OnSummary = func(summary string) {
				p.Send(tui.ConversationSummaryMsg{Summary: summary})
			}
			sum.OnSessionTitle = func(title string) {
				p.Send(tui.SessionTitleMsg{Title: title})
			}
			if err := sum.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not start summarizer: %v\n", err)
			} else {
				defer sum.Stop()
			}
		}
	}

	// Start a goroutine to periodically refresh session state
	// Poll regardless of initial load success so we can recover from errors
	if piledriverDir != "" && activeSession != "" {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				info, err := state.GetSessionInfo(piledriverDir, activeSession)
				if err == nil {
					p.Send(tui.StateUpdateMsg{SessionInfo: info})
				}
			}
		}()
	}

	// Start watcher
	if err := w.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting watcher: %v\n", err)
		os.Exit(1)
	}
	defer w.Stop()

	// Run TUI
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
