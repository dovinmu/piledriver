package tui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dovinmu/piledriver/pilemonkey/state"
	"github.com/dovinmu/piledriver/pilemonkey/store"
)

// ViewMode represents which view is active
type ViewMode int

const (
	ViewModeDiff ViewMode = iota
	ViewModeOverview
	ViewModeCritic
)

// ChangesetMsg is sent when a new changeset arrives
type ChangesetMsg struct {
	Changeset store.Changeset
}

// StateUpdateMsg is sent when the session state changes
type StateUpdateMsg struct {
	SessionInfo *state.SessionInfo
}

// ConversationSummaryMsg is sent when the conversation summary is updated
type ConversationSummaryMsg struct {
	Summary string
}

// SessionTitleMsg is sent when the session title is read
type SessionTitleMsg struct {
	Title string
}

// Model represents the TUI state
type Model struct {
	// Diff mode state
	store        *store.Store
	currentIndex int  // current position in changeset history (0 = oldest)
	scrollOffset int  // vertical scroll position within current diff
	liveMode     bool // auto-follow newest changes

	// View mode
	viewMode ViewMode

	// Overview mode state
	sessionInfo  *state.SessionInfo
	viewingPhase state.Phase // which phase's notes we're viewing
	editingNote  bool        // currently editing a note
	noteInput    string      // current note being edited
	noteInputPos int         // cursor position in note input

	// Critic mode state
	criticEntries     []state.CriticEntry // loaded critic exchanges
	criticIndex       int                 // current exchange being viewed (0 = oldest)
	criticScrollOffset int                // scroll position within current exchange

	// Shared state
	width   int
	height  int
	keys    KeyMap
	rootDir string

	// Conversation summarizer state
	conversationSummary string
	sessionTitle        string // Built-in summary from JSONL
}

// NewModel creates a new TUI model
func NewModel(s *store.Store, rootDir string) Model {
	return Model{
		store:        s,
		currentIndex: -1, // Will be set to newest on first changeset
		scrollOffset: 0,
		liveMode:     true,
		viewMode:     ViewModeOverview,
		viewingPhase: state.PhaseReconnaissance,
		keys:         DefaultKeyMap(),
		rootDir:      rootDir,
	}
}

// SetSessionInfo sets the session info for overview mode
func (m *Model) SetSessionInfo(info *state.SessionInfo) {
	m.sessionInfo = info
	if info != nil && info.State != nil {
		m.viewingPhase = info.State.CurrentPhase
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case ChangesetMsg:
		// New changeset arrived
		if m.liveMode && m.viewMode == ViewModeDiff {
			m.currentIndex = m.store.Size() - 1
			m.scrollOffset = 0
		}
		return m, nil

	case StateUpdateMsg:
		// Session state was updated
		m.sessionInfo = msg.SessionInfo
		if msg.SessionInfo != nil && msg.SessionInfo.State != nil {
			// Update viewing phase to current if we haven't navigated away
			if m.viewingPhase == state.PhaseReconnaissance || m.sessionInfo == nil {
				m.viewingPhase = msg.SessionInfo.State.CurrentPhase
			}
		}
		return m, nil

	case ConversationSummaryMsg:
		// Conversation summary was updated
		m.conversationSummary = msg.Summary
		return m, nil

	case SessionTitleMsg:
		// Session title was read
		m.sessionTitle = msg.Title
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle note editing mode first
	if m.editingNote {
		return m.handleNoteInput(msg)
	}

	// Global keys
	switch {
	case matchesKey(msg, m.keys.Quit):
		return m, tea.Quit

	case matchesKey(msg, m.keys.ToggleView):
		// Cycle through views: Diff → Overview → Critic → Diff
		switch m.viewMode {
		case ViewModeDiff:
			m.viewMode = ViewModeOverview
		case ViewModeOverview:
			m.viewMode = ViewModeCritic
			// Load critic log when entering critic mode
			if m.sessionInfo != nil && m.sessionInfo.Dir != "" {
				entries, _ := state.LoadCriticLog(m.sessionInfo.Dir)
				m.criticEntries = entries
				if len(entries) > 0 {
					m.criticIndex = len(entries) - 1 // Start at newest
				}
				m.criticScrollOffset = 0
			}
		case ViewModeCritic:
			m.viewMode = ViewModeDiff
			// When switching to diff mode in live mode, jump to newest changeset
			if m.liveMode {
				m.currentIndex = m.store.Size() - 1
				m.scrollOffset = 0
			}
		}
		return m, nil
	}

	// Mode-specific keys
	switch m.viewMode {
	case ViewModeDiff:
		return m.handleDiffKey(msg)
	case ViewModeOverview:
		return m.handleOverviewKey(msg)
	case ViewModeCritic:
		return m.handleCriticKey(msg)
	}
	return m, nil
}

func (m Model) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(msg, m.keys.PrevChange):
		if m.currentIndex > 0 {
			m.currentIndex--
			m.scrollOffset = 0
			m.liveMode = false
		}

	case matchesKey(msg, m.keys.NextChange):
		maxIndex := m.store.Size() - 1
		if m.currentIndex < maxIndex {
			m.currentIndex++
			m.scrollOffset = 0
			// Re-enable live mode if we reach the newest
			if m.currentIndex == maxIndex {
				m.liveMode = true
			}
		}

	case matchesKey(msg, m.keys.ScrollUp):
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}

	case matchesKey(msg, m.keys.ScrollDown):
		m.scrollOffset++

	case matchesKey(msg, m.keys.PageUp):
		contentHeight := m.height - 4 // Account for header and footer
		m.scrollOffset -= contentHeight
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}

	case matchesKey(msg, m.keys.PageDown):
		contentHeight := m.height - 4
		m.scrollOffset += contentHeight

	case matchesKey(msg, m.keys.Top):
		m.scrollOffset = 0

	case matchesKey(msg, m.keys.Bottom):
		// This will be clamped in the view
		m.scrollOffset = 99999

	case matchesKey(msg, m.keys.Live):
		// Jump to newest and enable live mode
		m.currentIndex = m.store.Size() - 1
		m.scrollOffset = 0
		m.liveMode = true
	}

	return m, nil
}

func (m Model) handleOverviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	phases := state.AllPhases()
	currentIdx := state.PhaseIndex(m.viewingPhase)

	switch {
	case matchesKey(msg, m.keys.PrevChange):
		// Navigate to previous phase
		if currentIdx > 0 {
			m.viewingPhase = phases[currentIdx-1]
		}

	case matchesKey(msg, m.keys.NextChange):
		// Navigate to next phase
		if currentIdx < len(phases)-1 {
			m.viewingPhase = phases[currentIdx+1]
		}

	case matchesKey(msg, m.keys.AddNote):
		// Start editing note for current viewing phase
		m.editingNote = true
		if m.sessionInfo != nil && m.sessionInfo.State != nil {
			m.noteInput = m.sessionInfo.State.GetNote(m.viewingPhase)
		} else {
			m.noteInput = ""
		}
		m.noteInputPos = len(m.noteInput)

	case matchesKey(msg, m.keys.ScrollUp):
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}

	case matchesKey(msg, m.keys.ScrollDown):
		m.scrollOffset++
	}

	return m, nil
}

func (m Model) handleCriticKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(msg, m.keys.PrevChange):
		// Navigate to previous exchange
		if m.criticIndex > 0 {
			m.criticIndex--
			m.criticScrollOffset = 0
		}

	case matchesKey(msg, m.keys.NextChange):
		// Navigate to next exchange
		if m.criticIndex < len(m.criticEntries)-1 {
			m.criticIndex++
			m.criticScrollOffset = 0
		}

	case matchesKey(msg, m.keys.ScrollUp):
		if m.criticScrollOffset > 0 {
			m.criticScrollOffset--
		}

	case matchesKey(msg, m.keys.ScrollDown):
		m.criticScrollOffset++

	case matchesKey(msg, m.keys.PageUp):
		contentHeight := m.height - 4
		m.criticScrollOffset -= contentHeight
		if m.criticScrollOffset < 0 {
			m.criticScrollOffset = 0
		}

	case matchesKey(msg, m.keys.PageDown):
		contentHeight := m.height - 4
		m.criticScrollOffset += contentHeight

	case matchesKey(msg, m.keys.Top):
		m.criticScrollOffset = 0

	case matchesKey(msg, m.keys.Bottom):
		m.criticScrollOffset = 99999

	case matchesKey(msg, m.keys.Live):
		// Jump to newest exchange
		if len(m.criticEntries) > 0 {
			m.criticIndex = len(m.criticEntries) - 1
			m.criticScrollOffset = 0
		}
	}

	return m, nil
}

func (m Model) handleNoteInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Save note
		if m.sessionInfo != nil && m.sessionInfo.Dir != "" {
			err := state.UpdateStateWithLock(m.sessionInfo.Dir, func(s *state.SessionState) error {
				s.SetNote(m.viewingPhase, m.noteInput)
				return nil
			})
			if err == nil && m.sessionInfo.State != nil {
				m.sessionInfo.State.SetNote(m.viewingPhase, m.noteInput)
			}
		}
		m.editingNote = false
		m.noteInput = ""

	case "esc":
		// Cancel editing
		m.editingNote = false
		m.noteInput = ""

	case "backspace":
		if len(m.noteInput) > 0 && m.noteInputPos > 0 {
			m.noteInput = m.noteInput[:m.noteInputPos-1] + m.noteInput[m.noteInputPos:]
			m.noteInputPos--
		}

	case "delete":
		if m.noteInputPos < len(m.noteInput) {
			m.noteInput = m.noteInput[:m.noteInputPos] + m.noteInput[m.noteInputPos+1:]
		}

	case "left":
		if m.noteInputPos > 0 {
			m.noteInputPos--
		}

	case "right":
		if m.noteInputPos < len(m.noteInput) {
			m.noteInputPos++
		}

	case "home":
		m.noteInputPos = 0

	case "end":
		m.noteInputPos = len(m.noteInput)

	default:
		// Insert character
		if len(msg.String()) == 1 || msg.Type == tea.KeySpace {
			char := msg.String()
			if msg.Type == tea.KeySpace {
				char = " "
			}
			m.noteInput = m.noteInput[:m.noteInputPos] + char + m.noteInput[m.noteInputPos:]
			m.noteInputPos++
		}
	}

	return m, nil
}

// GetCurrentChangeset returns the currently displayed changeset
func (m Model) GetCurrentChangeset() (store.Changeset, bool) {
	return m.store.Get(m.currentIndex)
}

// RelativePath returns the path relative to the root directory
func (m Model) RelativePath(path string) string {
	rel, err := filepath.Rel(m.rootDir, path)
	if err != nil {
		return path
	}
	return rel
}

// View implements tea.Model
func (m Model) View() string {
	return renderView(m)
}
