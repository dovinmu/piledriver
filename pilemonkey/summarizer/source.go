package summarizer

import "time"

// parsedMessage represents a message extracted from a conversation source
type parsedMessage struct {
	role string
	text string
}

// ConversationSource provides conversation data from an AI coding tool
type ConversationSource interface {
	// Name returns the source name (e.g. "claude", "opencode")
	Name() string
	// FindCurrent returns the most recent session for the project dir.
	FindCurrent() (sessionID, sessionTitle string, lastUpdated time.Time, err error)
	// ReadRecentMessages reads the last N messages from the given session.
	ReadRecentMessages(sessionID string, limit int) ([]parsedMessage, error)
	// HasChanged returns true if the session has been modified since the last check.
	HasChanged(sessionID string) (bool, error)
}

// SummaryGenerator produces a summary from a prompt string.
type SummaryGenerator interface {
	Summarize(prompt string) (string, error)
}
