package summarizer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConversationMessage represents a message from the Claude Code conversation log
type ConversationMessage struct {
	Type      string          `json:"type"` // "user" or "assistant"
	UUID      string          `json:"uuid"`
	Timestamp string          `json:"timestamp"`
	Message   json.RawMessage `json:"message"`
}

// MessageContent is the parsed message content (for assistant messages with array content)
type MessageContent struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // Can be string or []ContentItem
}

// ContentItem represents a content block in a message
type ContentItem struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result", "thinking", etc.
	Text string `json:"text,omitempty"`
}

// SummaryLine represents the first line of a JSONL conversation file
type SummaryLine struct {
	Type    string `json:"type"`
	Summary string `json:"summary"`
}

// Options configures the Summarizer.
type Options struct {
	SourcePreference string // "auto", "claude", or "opencode"
	Debug            bool
}

// Summarizer monitors AI conversation files and generates summaries.
type Summarizer struct {
	projectDir       string
	source           ConversationSource
	generator        SummaryGenerator
	currentSessionID string
	watcher          *fsnotify.Watcher // only used for Claude Code source

	mu           sync.RWMutex
	summary      string
	sessionTitle string

	// Callback when summary changes
	OnSummary func(summary string)

	// Callback when session title is read
	OnSessionTitle func(title string)

	// Configuration
	messagesToRead int
	quietPeriod    time.Duration
	debug          bool
}

// New creates a new Summarizer for the given project directory.
// It auto-detects the best conversation source, or uses the one specified in opts.
func New(projectDir string, opts Options) (*Summarizer, error) {
	var sources []ConversationSource

	switch opts.SourcePreference {
	case "claude":
		src, err := NewClaudeCodeSource(projectDir)
		if err != nil {
			return nil, fmt.Errorf("claude source: %w", err)
		}
		sources = []ConversationSource{src}
	case "opencode":
		src, err := NewOpenCodeSource(projectDir)
		if err != nil {
			return nil, fmt.Errorf("opencode source: %w", err)
		}
		sources = []ConversationSource{src}
	default: // "auto" or ""
		if src, err := NewClaudeCodeSource(projectDir); err == nil {
			sources = append(sources, src)
		}
		if src, err := NewOpenCodeSource(projectDir); err == nil {
			sources = append(sources, src)
		}
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no conversation sources found for %s", projectDir)
	}

	// Pick the source with the most recently updated session
	var bestSource ConversationSource
	var bestSessionID, bestTitle string
	var bestTime time.Time

	for _, src := range sources {
		sid, title, updated, err := src.FindCurrent()
		if err != nil {
			if opts.Debug {
				fmt.Fprintf(os.Stderr, "[summarizer] source %s: %v\n", src.Name(), err)
			}
			continue
		}
		if opts.Debug {
			fmt.Fprintf(os.Stderr, "[summarizer] source %s: session=%s updated=%s\n", src.Name(), sid, updated.Format(time.RFC3339))
		}
		if updated.After(bestTime) {
			bestTime = updated
			bestSource = src
			bestSessionID = sid
			bestTitle = title
		}
	}

	if bestSource == nil {
		return nil, fmt.Errorf("no active sessions found for %s", projectDir)
	}

	if opts.Debug {
		fmt.Fprintf(os.Stderr, "[summarizer] selected source: %s (session: %s)\n", bestSource.Name(), bestSessionID)
	}

	s := &Summarizer{
		projectDir:       projectDir,
		source:           bestSource,
		generator:        &ClaudeSummaryGenerator{},
		currentSessionID: bestSessionID,
		messagesToRead:   30,
		quietPeriod:      20 * time.Second,
		debug:            opts.Debug,
	}

	if bestTitle != "" {
		s.sessionTitle = bestTitle
	}

	return s, nil
}

// Start begins monitoring the conversation source.
func (s *Summarizer) Start() error {
	// Set up fsnotify only for Claude Code source
	if ccSrc, ok := s.source.(*ClaudeCodeSource); ok {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			s.debugLog("failed to create fsnotify watcher, will use polling only: %v", err)
		} else {
			s.watcher = watcher
			if err := watcher.Add(ccSrc.ConversationDir()); err != nil {
				watcher.Close()
				s.watcher = nil
				s.debugLog("failed to watch directory, will use polling only: %v", err)
			}
		}
	}

	// Start the monitoring goroutine
	go s.monitor()

	// Fire initial session title callback and generate initial summary
	go func() {
		if s.sessionTitle != "" && s.OnSessionTitle != nil {
			s.OnSessionTitle(s.sessionTitle)
		}
		s.generateSummary()
	}()

	return nil
}

// Stop stops the summarizer.
func (s *Summarizer) Stop() {
	if s.watcher != nil {
		s.watcher.Close()
	}
}

// GetSummary returns the current summary.
func (s *Summarizer) GetSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summary
}

// GetSessionTitle returns the session title.
func (s *Summarizer) GetSessionTitle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionTitle
}

// SetDebug enables or disables debug logging.
func (s *Summarizer) SetDebug(enabled bool) {
	s.debug = enabled
}

func (s *Summarizer) debugLog(format string, args ...interface{}) {
	if s.debug {
		fmt.Fprintf(os.Stderr, "[summarizer] "+format+"\n", args...)
	}
}

// monitor watches for changes and triggers summary after a quiet period.
func (s *Summarizer) monitor() {
	var quietTimer *time.Timer
	var lastEventTime time.Time
	var pendingSummary bool
	var timerMu sync.Mutex

	s.debugLog("starting monitor, source: %s, session: %s", s.source.Name(), s.currentSessionID)

	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()

	// Periodically check for newer sessions (in case a new session starts)
	sessionRefreshTicker := time.NewTicker(10 * time.Second)
	defer sessionRefreshTicker.Stop()

	handleChange := func(source string) {
		timerMu.Lock()
		lastEventTime = time.Now()
		pendingSummary = true

		if quietTimer != nil {
			quietTimer.Stop()
		}
		quietTimer = time.AfterFunc(s.quietPeriod, func() {
			timerMu.Lock()
			shouldGenerate := pendingSummary && time.Since(lastEventTime) >= s.quietPeriod
			if shouldGenerate {
				pendingSummary = false
			}
			timerMu.Unlock()

			if shouldGenerate {
				s.debugLog("quiet period elapsed, generating summary")
				s.generateSummary()
			}
		})
		timerMu.Unlock()
	}

	checkForChanges := func(source string) {
		changed, err := s.source.HasChanged(s.currentSessionID)
		if err != nil {
			return
		}
		if changed {
			s.debugLog("[%s] change detected in session %s", source, s.currentSessionID)
			handleChange(source)
		}
	}

	refreshSession := func() {
		sid, title, _, err := s.source.FindCurrent()
		if err != nil {
			return
		}
		if sid != s.currentSessionID {
			s.debugLog("new session detected: %s (was %s)", sid, s.currentSessionID)
			s.currentSessionID = sid
			if title != "" {
				s.mu.Lock()
				s.sessionTitle = title
				s.mu.Unlock()
				if s.OnSessionTitle != nil {
					s.OnSessionTitle(title)
				}
			}
			// Generate summary for the new session
			s.generateSummary()
		}
	}

	if s.watcher != nil {
		// Claude Code path: use fsnotify + polling fallback
		for {
			select {
			case event, ok := <-s.watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					checkForChanges("fsnotify")
				}

			case <-pollTicker.C:
				checkForChanges("poll")

			case <-sessionRefreshTicker.C:
				refreshSession()

			case err, ok := <-s.watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "Summarizer watcher error: %v\n", err)
			}
		}
	} else {
		// Polling-only path (opencode or fsnotify unavailable)
		for {
			select {
			case <-pollTicker.C:
				checkForChanges("poll")

			case <-sessionRefreshTicker.C:
				refreshSession()
			}
		}
	}
}

// generateSummary reads recent messages and generates a summary.
func (s *Summarizer) generateSummary() {
	s.debugLog("generateSummary called for session: %s", s.currentSessionID)

	messages, err := s.source.ReadRecentMessages(s.currentSessionID, s.messagesToRead)
	if err != nil {
		s.debugLog("error reading messages: %v", err)
		return
	}

	s.debugLog("found %d messages with text content", len(messages))

	if len(messages) == 0 {
		s.debugLog("no messages to summarize")
		return
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString(`<task>
Summarize what we're trying to accomplish in this conversation.
1-2 sentences max. High-level goal, not play-by-play details.
Plain text only, no markdown, no offers to help.
</task>

<conversation>
`)

	for _, msg := range messages {
		promptBuilder.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.role, msg.text))
	}
	promptBuilder.WriteString("</conversation>\n\nSummary:")

	prompt := promptBuilder.String()

	s.debugLog("calling summarizer with %d chars of prompt", len(prompt))
	summary, err := s.generator.Summarize(prompt)
	if err != nil {
		s.debugLog("summarizer call failed: %v", err)
		return
	}

	s.debugLog("got summary: %s", summary)

	s.mu.Lock()
	s.summary = summary
	s.mu.Unlock()

	if s.OnSummary != nil {
		s.OnSummary(summary)
	}
}
