package summarizer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// Summarizer monitors Claude Code conversation files and generates summaries
type Summarizer struct {
	projectDir      string
	conversationDir string
	watcher         *fsnotify.Watcher
	currentFile     string
	lastModTime     time.Time
	lastFileSize    int64 // Track file size to detect actual changes

	mu           sync.RWMutex
	summary      string
	sessionTitle string // Built-in summary from JSONL first line

	// Callback when summary changes
	OnSummary func(summary string)

	// Callback when session title is read
	OnSessionTitle func(title string)

	// Configuration
	messagesToRead int
	quietPeriod    time.Duration // How long to wait after last write before summarizing
	debug          bool          // Enable debug logging
}

// New creates a new Summarizer for the given project directory
func New(projectDir string) (*Summarizer, error) {
	// Convert project path to Claude Code's conversation directory format
	// Claude Code encodes paths by replacing / and _ with -
	// e.g., /home/rowan/Documents/piledriver -> -home-rowan-Documents-piledriver
	// e.g., /home/rowan/kindle_rss_reader -> -home-rowan-kindle-rss-reader
	absPath, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project path: %w", err)
	}

	encodedPath := strings.ReplaceAll(absPath, "/", "-")
	encodedPath = strings.ReplaceAll(encodedPath, "_", "-")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	conversationDir := filepath.Join(homeDir, ".claude", "projects", encodedPath)

	// Check if directory exists
	if _, err := os.Stat(conversationDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("conversation directory not found: %s", conversationDir)
	}

	return &Summarizer{
		projectDir:      projectDir,
		conversationDir: conversationDir,
		messagesToRead:  30,               // Read last 30 lines to find messages with text content
		quietPeriod:     20 * time.Second, // Wait 20 seconds of no writes before summarizing
	}, nil
}

// Start begins monitoring the conversation directory
func (s *Summarizer) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	s.watcher = watcher

	// Find the current conversation file
	if err := s.findCurrentConversation(); err != nil {
		return err
	}

	// Watch the conversation directory
	if err := watcher.Add(s.conversationDir); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Start the monitoring goroutine
	go s.monitor()

	// Read session title and generate initial summary
	go func() {
		s.readSessionTitle()
		s.generateSummary()
	}()

	return nil
}

// Stop stops the summarizer
func (s *Summarizer) Stop() {
	if s.watcher != nil {
		s.watcher.Close()
	}
}

// GetSummary returns the current summary
func (s *Summarizer) GetSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summary
}

// SetDebug enables or disables debug logging
func (s *Summarizer) SetDebug(enabled bool) {
	s.debug = enabled
}

func (s *Summarizer) debugLog(format string, args ...interface{}) {
	if s.debug {
		fmt.Fprintf(os.Stderr, "[summarizer] "+format+"\n", args...)
	}
}

// readSessionTitle reads the built-in summary from the first line of the conversation file
func (s *Summarizer) readSessionTitle() {
	if s.currentFile == "" {
		return
	}

	file, err := os.Open(s.currentFile)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		var summaryLine SummaryLine
		if err := json.Unmarshal([]byte(scanner.Text()), &summaryLine); err == nil {
			if summaryLine.Type == "summary" && summaryLine.Summary != "" {
				s.mu.Lock()
				s.sessionTitle = summaryLine.Summary
				s.mu.Unlock()

				s.debugLog("read session title: %s", summaryLine.Summary)

				if s.OnSessionTitle != nil {
					s.OnSessionTitle(summaryLine.Summary)
				}
			}
		}
	}
}

// GetSessionTitle returns the session title
func (s *Summarizer) GetSessionTitle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionTitle
}

// findCurrentConversation finds the most recently modified conversation file
func (s *Summarizer) findCurrentConversation() error {
	entries, err := os.ReadDir(s.conversationDir)
	if err != nil {
		return fmt.Errorf("failed to read conversation directory: %w", err)
	}

	var latestFile string
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Only look at main conversation files (UUIDs), not agent files
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		if strings.HasPrefix(name, "agent-") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = filepath.Join(s.conversationDir, name)
		}
	}

	if latestFile == "" {
		return fmt.Errorf("no conversation files found")
	}

	s.currentFile = latestFile
	s.lastModTime = latestTime

	// Get initial file size
	if info, err := os.Stat(latestFile); err == nil {
		s.lastFileSize = info.Size()
	}

	return nil
}

// monitor watches for file changes and triggers summary after a quiet period
// Uses both fsnotify and polling as a fallback since fsnotify can be unreliable
func (s *Summarizer) monitor() {
	var quietTimer *time.Timer
	var lastEventTime time.Time
	var pendingSummary bool
	var timerMu sync.Mutex

	s.debugLog("starting monitor, watching: %s", s.conversationDir)
	s.debugLog("initial file: %s (size: %d)", s.currentFile, s.lastFileSize)

	// Also poll every 2 seconds as a fallback
	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()

	checkForChanges := func(source string) {
		// Find the most recent conversation file
		entries, err := os.ReadDir(s.conversationDir)
		if err != nil {
			return
		}

		var latestFile string
		var latestTime time.Time
		var latestSize int64

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".jsonl") || strings.HasPrefix(name, "agent-") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = filepath.Join(s.conversationDir, name)
				latestSize = info.Size()
			}
		}

		if latestFile == "" {
			return
		}

		// Check if there's new content
		isNewContent := latestSize != s.lastFileSize || latestTime.After(s.lastModTime)
		if isNewContent {
			s.debugLog("[%s] change detected: %s (size: %d -> %d)", source, filepath.Base(latestFile), s.lastFileSize, latestSize)
			s.currentFile = latestFile
			s.lastModTime = latestTime
			s.lastFileSize = latestSize

			timerMu.Lock()
			lastEventTime = time.Now()
			pendingSummary = true

			// Reset the quiet period timer
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
	}

	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			// Check on any event to the directory
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				checkForChanges("fsnotify")
			}

		case <-pollTicker.C:
			// Polling fallback
			checkForChanges("poll")

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "Summarizer watcher error: %v\n", err)
		}
	}
}

// generateSummary reads recent messages and generates a summary
func (s *Summarizer) generateSummary() {
	s.debugLog("generateSummary called, reading from: %s", s.currentFile)

	messages, err := s.readRecentMessages()
	if err != nil {
		s.debugLog("error reading messages: %v", err)
		return
	}

	s.debugLog("found %d messages with text content", len(messages))

	if len(messages) == 0 {
		s.debugLog("no messages to summarize")
		return
	}

	// Build the prompt for Claude
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

	// Call Claude with haiku model
	s.debugLog("calling claude with %d chars of prompt", len(prompt))
	summary, err := s.callClaude(prompt)
	if err != nil {
		s.debugLog("claude call failed: %v", err)
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

type parsedMessage struct {
	role string
	text string
}

// readRecentMessages reads the last N messages from the conversation file
func (s *Summarizer) readRecentMessages() ([]parsedMessage, error) {
	if s.currentFile == "" {
		return nil, fmt.Errorf("no conversation file")
	}

	file, err := os.Open(s.currentFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read all lines (not efficient for large files, but conversation files are typically manageable)
	var lines []string
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Get the last N lines
	start := len(lines) - s.messagesToRead
	if start < 0 {
		start = 0
	}
	recentLines := lines[start:]

	// Parse messages
	var messages []parsedMessage
	for _, line := range recentLines {
		var msg ConversationMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		// Skip non-message types (summary, file-history-snapshot, etc.)
		if msg.Type != "user" && msg.Type != "assistant" {
			continue
		}

		// Extract text content from the message
		var content MessageContent
		if err := json.Unmarshal(msg.Message, &content); err != nil {
			continue
		}

		var textParts []string

		// Try to parse content as a string first (user messages)
		var contentStr string
		if err := json.Unmarshal(content.Content, &contentStr); err == nil {
			// Content is a string
			if contentStr != "" && !strings.Contains(contentStr, "<system-reminder>") {
				textParts = append(textParts, contentStr)
			}
		} else {
			// Try to parse content as an array (assistant messages)
			var contentItems []ContentItem
			if err := json.Unmarshal(content.Content, &contentItems); err == nil {
				for _, item := range contentItems {
					// Only include text content, skip thinking, tool_use, tool_result
					if item.Type == "text" && item.Text != "" {
						// Skip system reminders
						if strings.Contains(item.Text, "<system-reminder>") {
							continue
						}
						textParts = append(textParts, item.Text)
					}
				}
			}
		}

		if len(textParts) > 0 {
			role := msg.Type
			if role == "" {
				role = content.Role
			}
			// Truncate long messages
			text := strings.Join(textParts, " ")
			if len(text) > 500 {
				text = text[:500] + "..."
			}
			messages = append(messages, parsedMessage{role: role, text: text})
		}
	}

	// Limit to the last 6 messages with actual text content
	if len(messages) > 6 {
		messages = messages[len(messages)-6:]
	}

	return messages, nil
}

// callClaude invokes claude -p with the haiku model, passing prompt via stdin
// Runs from /tmp to avoid loading project-specific context
func (s *Summarizer) callClaude(prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", "--model", "haiku")
	cmd.Dir = "/tmp" // Run from neutral directory to avoid project context
	cmd.Stdin = strings.NewReader(prompt)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to call claude: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
