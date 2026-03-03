package summarizer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeCodeSource reads conversations from Claude Code's JSONL files.
type ClaudeCodeSource struct {
	conversationDir string
	lastModTime     time.Time
	lastFileSize    int64
}

// NewClaudeCodeSource creates a source for Claude Code conversations.
// projectDir is the absolute path to the project being monitored.
func NewClaudeCodeSource(projectDir string) (*ClaudeCodeSource, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Claude Code encodes paths by replacing / and _ with -
	encodedPath := strings.ReplaceAll(projectDir, "/", "-")
	encodedPath = strings.ReplaceAll(encodedPath, "_", "-")

	conversationDir := filepath.Join(homeDir, ".claude", "projects", encodedPath)

	if _, err := os.Stat(conversationDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("conversation directory not found: %s", conversationDir)
	}

	return &ClaudeCodeSource{
		conversationDir: conversationDir,
	}, nil
}

func (s *ClaudeCodeSource) Name() string { return "claude" }

// ConversationDir returns the path to the conversation directory (used by Summarizer for fsnotify).
func (s *ClaudeCodeSource) ConversationDir() string { return s.conversationDir }

func (s *ClaudeCodeSource) FindCurrent() (sessionID, sessionTitle string, lastUpdated time.Time, err error) {
	entries, err := os.ReadDir(s.conversationDir)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to read conversation directory: %w", err)
	}

	var latestFile string
	var latestTime time.Time

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
		}
	}

	if latestFile == "" {
		return "", "", time.Time{}, fmt.Errorf("no conversation files found")
	}

	// Read session title from JSONL first line
	title := readJSONLSessionTitle(latestFile)

	return latestFile, title, latestTime, nil
}

func (s *ClaudeCodeSource) ReadRecentMessages(sessionID string, limit int) ([]parsedMessage, error) {
	file, err := os.Open(sessionID)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Get the last N lines
	start := len(lines) - limit
	if start < 0 {
		start = 0
	}
	recentLines := lines[start:]

	var messages []parsedMessage
	for _, line := range recentLines {
		var msg ConversationMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.Type != "user" && msg.Type != "assistant" {
			continue
		}

		var content MessageContent
		if err := json.Unmarshal(msg.Message, &content); err != nil {
			continue
		}

		var textParts []string

		var contentStr string
		if err := json.Unmarshal(content.Content, &contentStr); err == nil {
			if contentStr != "" && !strings.Contains(contentStr, "<system-reminder>") {
				textParts = append(textParts, contentStr)
			}
		} else {
			var contentItems []ContentItem
			if err := json.Unmarshal(content.Content, &contentItems); err == nil {
				for _, item := range contentItems {
					if item.Type == "text" && item.Text != "" {
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

func (s *ClaudeCodeSource) HasChanged(sessionID string) (bool, error) {
	info, err := os.Stat(sessionID)
	if err != nil {
		return false, err
	}

	changed := info.Size() != s.lastFileSize || info.ModTime().After(s.lastModTime)
	if changed {
		s.lastModTime = info.ModTime()
		s.lastFileSize = info.Size()
	}
	return changed, nil
}

// readJSONLSessionTitle reads the summary from the first line of a JSONL conversation file.
func readJSONLSessionTitle(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		var sl SummaryLine
		if err := json.Unmarshal([]byte(scanner.Text()), &sl); err == nil {
			if sl.Type == "summary" {
				return sl.Summary
			}
		}
	}
	return ""
}
