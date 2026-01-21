package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Phase represents a piledriver workflow phase
type Phase string

const (
	PhaseReconnaissance Phase = "RECONNAISSANCE"
	PhaseScoping        Phase = "SCOPING"
	PhaseAssumptions    Phase = "ASSUMPTIONS"
	PhaseVerification   Phase = "VERIFICATION"
	PhaseReport         Phase = "REPORT"

	// PhaseIdle is deprecated but kept for backwards compatibility with old state files
	PhaseIdle Phase = "IDLE"
)

// AllPhases returns all phases in order (excluding deprecated IDLE)
func AllPhases() []Phase {
	return []Phase{
		PhaseReconnaissance,
		PhaseScoping,
		PhaseAssumptions,
		PhaseVerification,
		PhaseReport,
	}
}

// NormalizePhase converts legacy IDLE phase to RECONNAISSANCE
func NormalizePhase(p Phase) Phase {
	if p == PhaseIdle {
		return PhaseReconnaissance
	}
	return p
}

// PhaseIndex returns the index of a phase (0-4)
func PhaseIndex(p Phase) int {
	for i, phase := range AllPhases() {
		if phase == p {
			return i
		}
	}
	return -1
}

// PhaseEntry records when a phase was entered
type PhaseEntry struct {
	Phase   Phase     `json:"phase"`
	Entered time.Time `json:"entered"`
}

// TLCResult records the result of a TLC model check
type TLCResult struct {
	Timestamp      time.Time `json:"timestamp"`
	SanyOnly       bool      `json:"sany_only"`
	Success        bool      `json:"success"`
	StatesGenerated int      `json:"states_generated"`
	DistinctStates int       `json:"distinct_states"`
	Violations     []string  `json:"violations,omitempty"`
}

// SessionState represents the state of a piledriver session
type SessionState struct {
	Session      string           `json:"session"`
	CurrentPhase Phase            `json:"current_phase"`
	PhaseHistory []PhaseEntry     `json:"phase_history"`
	PhaseNotes   map[Phase]string `json:"phase_notes"`
	TLCResults   []TLCResult      `json:"tlc_results,omitempty"`
	Summary      string           `json:"summary,omitempty"` // High-level task description
	Created      time.Time        `json:"created"`
	LastModified time.Time        `json:"last_modified"`
}

// NewSessionState creates a new session state with defaults
func NewSessionState(sessionName string) *SessionState {
	now := time.Now()
	return &SessionState{
		Session:      sessionName,
		CurrentPhase: PhaseReconnaissance,
		PhaseHistory: []PhaseEntry{
			{Phase: PhaseReconnaissance, Entered: now},
		},
		PhaseNotes:   make(map[Phase]string),
		Created:      now,
		LastModified: now,
	}
}

// StateFilePath returns the path to the state.json file for a session
func StateFilePath(sessionDir string) string {
	return filepath.Join(sessionDir, "state.json")
}

// LoadState reads the session state from disk
// Returns nil if the file doesn't exist
func LoadState(sessionDir string) (*SessionState, error) {
	path := StateFilePath(sessionDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	// Ensure PhaseNotes is initialized
	if state.PhaseNotes == nil {
		state.PhaseNotes = make(map[Phase]string)
	}

	return &state, nil
}

// SaveState writes the session state to disk
func SaveState(sessionDir string, state *SessionState) error {
	state.LastModified = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	path := StateFilePath(sessionDir)
	return os.WriteFile(path, data, 0644)
}

// TransitionPhase moves to a new phase and records it in history
func (s *SessionState) TransitionPhase(newPhase Phase) {
	s.CurrentPhase = newPhase
	s.PhaseHistory = append(s.PhaseHistory, PhaseEntry{
		Phase:   newPhase,
		Entered: time.Now(),
	})
	s.LastModified = time.Now()
}

// SetNote sets a note for a specific phase
func (s *SessionState) SetNote(phase Phase, note string) {
	if s.PhaseNotes == nil {
		s.PhaseNotes = make(map[Phase]string)
	}
	s.PhaseNotes[phase] = note
	s.LastModified = time.Now()
}

// GetNote returns the note for a specific phase
func (s *SessionState) GetNote(phase Phase) string {
	if s.PhaseNotes == nil {
		return ""
	}
	return s.PhaseNotes[phase]
}

// HasVisitedPhase returns true if the phase has been visited
func (s *SessionState) HasVisitedPhase(phase Phase) bool {
	for _, entry := range s.PhaseHistory {
		if entry.Phase == phase {
			return true
		}
	}
	return false
}

// LatestTLCResult returns the most recent TLC result, or nil if none exist
func (s *SessionState) LatestTLCResult() *TLCResult {
	if len(s.TLCResults) == 0 {
		return nil
	}
	return &s.TLCResults[len(s.TLCResults)-1]
}
