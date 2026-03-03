package summarizer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// OpenCodeSource reads conversations from opencode's JSON file storage.
type OpenCodeSource struct {
	storageDir  string // ~/.local/share/opencode/storage
	projectDir  string // absolute path to the project
	lastMsgDirModTime time.Time
}

// opencode JSON structures

type ocSession struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectID"`
	Directory string   `json:"directory"`
	Title     string   `json:"title"`
	Time      ocTime   `json:"time"`
}

type ocTime struct {
	Created int64 `json:"created"` // milliseconds
	Updated int64 `json:"updated"` // milliseconds
}

type ocMessage struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Role      string `json:"role"`
	Time      ocTime `json:"time"`
}

type ocPart struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	Type      string `json:"type"` // "text", "tool-invocation", etc.
	Text      string `json:"text"`
	Synthetic bool   `json:"synthetic"`
}

// NewOpenCodeSource creates a source for opencode conversations.
func NewOpenCodeSource(projectDir string) (*OpenCodeSource, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storageDir := filepath.Join(homeDir, ".local", "share", "opencode", "storage")
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("opencode storage not found: %s", storageDir)
	}

	return &OpenCodeSource{
		storageDir: storageDir,
		projectDir: projectDir,
	}, nil
}

func (s *OpenCodeSource) Name() string { return "opencode" }

func (s *OpenCodeSource) FindCurrent() (sessionID, sessionTitle string, lastUpdated time.Time, err error) {
	// Scan all project directories for sessions matching our project path
	sessionDir := filepath.Join(s.storageDir, "session")
	projectDirs, err := os.ReadDir(sessionDir)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to read session directory: %w", err)
	}

	var bestSession ocSession
	var bestTime time.Time

	for _, projEntry := range projectDirs {
		if !projEntry.IsDir() {
			continue
		}

		projPath := filepath.Join(sessionDir, projEntry.Name())
		sessionFiles, err := os.ReadDir(projPath)
		if err != nil {
			continue
		}

		for _, sf := range sessionFiles {
			if sf.IsDir() || !strings.HasSuffix(sf.Name(), ".json") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(projPath, sf.Name()))
			if err != nil {
				continue
			}

			var sess ocSession
			if err := json.Unmarshal(data, &sess); err != nil {
				continue
			}

			if sess.Directory != s.projectDir {
				continue
			}

			updated := time.UnixMilli(sess.Time.Updated)
			if updated.After(bestTime) {
				bestTime = updated
				bestSession = sess
			}
		}
	}

	if bestSession.ID == "" {
		return "", "", time.Time{}, fmt.Errorf("no opencode sessions found for %s", s.projectDir)
	}

	return bestSession.ID, bestSession.Title, bestTime, nil
}

func (s *OpenCodeSource) ReadRecentMessages(sessionID string, limit int) ([]parsedMessage, error) {
	// Read messages for this session
	msgDir := filepath.Join(s.storageDir, "message", sessionID)
	msgFiles, err := os.ReadDir(msgDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages: %w", err)
	}

	// Parse and sort messages by creation time
	type msgWithTime struct {
		msg  ocMessage
		time int64
	}
	var msgs []msgWithTime
	for _, f := range msgFiles {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(msgDir, f.Name()))
		if err != nil {
			continue
		}
		var m ocMessage
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		msgs = append(msgs, msgWithTime{msg: m, time: m.Time.Created})
	}

	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].time < msgs[j].time
	})

	// Take the last `limit` messages
	start := len(msgs) - limit
	if start < 0 {
		start = 0
	}
	recentMsgs := msgs[start:]

	// For each message, read its text parts
	var messages []parsedMessage
	for _, mwt := range recentMsgs {
		partDir := filepath.Join(s.storageDir, "part", mwt.msg.ID)
		partFiles, err := os.ReadDir(partDir)
		if err != nil {
			continue
		}

		var textParts []string
		for _, pf := range partFiles {
			if pf.IsDir() || !strings.HasSuffix(pf.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(partDir, pf.Name()))
			if err != nil {
				continue
			}
			var part ocPart
			if err := json.Unmarshal(data, &part); err != nil {
				continue
			}
			if part.Type == "text" && !part.Synthetic && part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		}

		if len(textParts) > 0 {
			text := strings.Join(textParts, " ")
			if len(text) > 500 {
				text = text[:500] + "..."
			}
			messages = append(messages, parsedMessage{role: mwt.msg.Role, text: text})
		}
	}

	// Limit to the last 6 messages with actual text content
	if len(messages) > 6 {
		messages = messages[len(messages)-6:]
	}

	return messages, nil
}

func (s *OpenCodeSource) HasChanged(sessionID string) (bool, error) {
	// Check if the message directory mtime has changed (new message files = updated mtime)
	msgDir := filepath.Join(s.storageDir, "message", sessionID)
	info, err := os.Stat(msgDir)
	if err != nil {
		return false, err
	}

	changed := info.ModTime().After(s.lastMsgDirModTime)
	if changed {
		s.lastMsgDirModTime = info.ModTime()
	}
	return changed, nil
}
