package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dovinmu/piledriver/pilemonkey/state"
)

var (
	// Overview-specific styles
	phaseActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50fa7b")).
				Bold(true)

	phaseCurrentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ff79c6")).
				Bold(true)

	phaseInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6272a4"))

	phaseViewingStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#44475a")).
				Foreground(lipgloss.Color("#f8f8f2")).
				Bold(true).
				Padding(0, 1)

	fileExistsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50fa7b"))

	fileMissingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6272a4"))

	sectionTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8be9fd")).
				Bold(true)

	noteBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272a4")).
			Padding(0, 1)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#ff79c6")).
			Padding(0, 1)
)

func renderOverview(m Model) string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	header := renderOverviewHeader(m)
	b.WriteString(header)
	b.WriteString("\n")

	// Content
	contentHeight := m.height - 3
	content := renderOverviewContent(m, contentHeight)
	b.WriteString(content)

	// Footer
	footer := renderOverviewFooter(m)
	b.WriteString(footer)

	return b.String()
}

func renderOverviewHeader(m Model) string {
	var sessionName, phaseName string

	if m.sessionInfo != nil {
		sessionName = m.sessionInfo.Name
		if m.sessionInfo.State != nil {
			phaseName = string(m.sessionInfo.State.CurrentPhase)
		} else {
			phaseName = "UNKNOWN"
		}
	} else {
		sessionName = "No session"
		phaseName = "-"
	}

	header := fmt.Sprintf("OVERVIEW │ %s │ %s", sessionName, phaseName)
	return headerStyle.Width(m.width).Render(header)
}

func renderOverviewContent(m Model, height int) string {
	var lines []string

	lines = append(lines, "")

	// Phase progress
	lines = append(lines, sectionTitleStyle.Render("  Phase Progress"))
	lines = append(lines, "")
	lines = append(lines, renderPhaseProgress(m)...)
	lines = append(lines, "")

	// File checklist
	lines = append(lines, sectionTitleStyle.Render("  Session Files"))
	lines = append(lines, "")
	lines = append(lines, renderFileChecklist(m)...)
	lines = append(lines, "")

	// Reproducers
	lines = append(lines, sectionTitleStyle.Render("  Reproducers"))
	lines = append(lines, "")
	lines = append(lines, renderReproducerSummary(m))
	lines = append(lines, "")

	// TLC Status
	lines = append(lines, sectionTitleStyle.Render("  TLC Status"))
	lines = append(lines, "")
	lines = append(lines, renderTLCStatus(m))
	lines = append(lines, "")

	// Notes section
	lines = append(lines, sectionTitleStyle.Render(fmt.Sprintf("  Notes for %s", m.viewingPhase)))
	lines = append(lines, "")
	lines = append(lines, renderNotes(m)...)
	lines = append(lines, "")

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:height], "\n") + "\n"
}

func renderPhaseProgress(m Model) []string {
	phases := state.AllPhases()
	var currentPhase state.Phase
	visited := make(map[state.Phase]bool)

	if m.sessionInfo != nil && m.sessionInfo.State != nil {
		currentPhase = m.sessionInfo.State.CurrentPhase
		for _, entry := range m.sessionInfo.State.PhaseHistory {
			visited[entry.Phase] = true
		}
	}

	var parts []string
	for _, phase := range phases {
		var indicator string
		var style lipgloss.Style

		if phase == m.viewingPhase {
			// Currently viewing this phase's notes
			indicator = "◉"
			style = phaseViewingStyle
		} else if phase == currentPhase {
			// Current active phase
			indicator = "●"
			style = phaseCurrentStyle
		} else if visited[phase] {
			// Visited phase
			indicator = "●"
			style = phaseActiveStyle
		} else {
			// Not yet reached
			indicator = "○"
			style = phaseInactiveStyle
		}

		parts = append(parts, style.Render(fmt.Sprintf("%s %s", indicator, phase)))
	}

	// Arrange phases in two rows for better display
	var lines []string
	line1 := "    " + strings.Join(parts[:3], " → ")
	line2 := "    " + strings.Join(parts[3:], " → ")
	lines = append(lines, line1)
	lines = append(lines, "         ↓")
	lines = append(lines, line2)

	return lines
}

func renderFileChecklist(m Model) []string {
	type fileCheck struct {
		name   string
		exists bool
	}

	var files []fileCheck
	if m.sessionInfo != nil {
		files = []fileCheck{
			{"boundary.md", m.sessionInfo.Files.Boundary},
			{"assumptions.md", m.sessionInfo.Files.Assumptions},
			{"model.tla", m.sessionInfo.Files.ModelTLA},
			{"model.cfg", m.sessionInfo.Files.ModelCfg},
			{"probe.md", m.sessionInfo.Files.Probe},
		}
	} else {
		files = []fileCheck{
			{"boundary.md", false},
			{"assumptions.md", false},
			{"model.tla", false},
			{"model.cfg", false},
			{"probe.md", false},
		}
	}

	var line1Parts, line2Parts []string
	for i, f := range files {
		var icon, styled string
		if f.exists {
			icon = "✓"
			styled = fileExistsStyle.Render(fmt.Sprintf("%s %s", icon, f.name))
		} else {
			icon = "○"
			styled = fileMissingStyle.Render(fmt.Sprintf("%s %s", icon, f.name))
		}

		if i < 3 {
			line1Parts = append(line1Parts, styled)
		} else {
			line2Parts = append(line2Parts, styled)
		}
	}

	return []string{
		"    " + strings.Join(line1Parts, "   "),
		"    " + strings.Join(line2Parts, "   "),
	}
}

func renderReproducerSummary(m Model) string {
	if m.sessionInfo == nil {
		return "    No session loaded"
	}

	total, passing, failing, notRun := m.sessionInfo.ReproducerSummary()

	if total == 0 {
		return "    No reproducers created"
	}

	parts := []string{fmt.Sprintf("%d total", total)}
	if passing > 0 {
		parts = append(parts, fileExistsStyle.Render(fmt.Sprintf("%d passing", passing)))
	}
	if failing > 0 {
		parts = append(parts, removedLineStyle.Render(fmt.Sprintf("%d failing", failing)))
	}
	if notRun > 0 {
		parts = append(parts, fileMissingStyle.Render(fmt.Sprintf("%d not run", notRun)))
	}

	return "    " + strings.Join(parts, ", ")
}

func renderTLCStatus(m Model) string {
	if m.sessionInfo == nil || m.sessionInfo.State == nil {
		return "    No session loaded"
	}

	result := m.sessionInfo.State.LatestTLCResult()
	if result == nil {
		return "    " + fileMissingStyle.Render("No TLC runs yet")
	}

	// Format: "TLC: 1234 states, no violations" or "TLC: 1234 states, 2 violations"
	var status string
	if result.SanyOnly {
		if result.Success {
			status = fileExistsStyle.Render("Syntax OK")
		} else {
			status = removedLineStyle.Render("Syntax errors")
		}
	} else {
		stateInfo := fmt.Sprintf("%d states", result.StatesGenerated)
		if result.DistinctStates > 0 {
			stateInfo = fmt.Sprintf("%d states (%d distinct)", result.StatesGenerated, result.DistinctStates)
		}

		if len(result.Violations) > 0 {
			status = fmt.Sprintf("%s, %s", stateInfo, removedLineStyle.Render(fmt.Sprintf("%d violations", len(result.Violations))))
		} else if result.Success {
			status = fmt.Sprintf("%s, %s", stateInfo, fileExistsStyle.Render("no violations"))
		} else {
			status = fmt.Sprintf("%s, %s", stateInfo, removedLineStyle.Render("errors"))
		}
	}

	return "    " + status
}

func renderNotes(m Model) []string {
	if m.editingNote {
		return renderNoteInput(m)
	}

	var note string
	if m.sessionInfo != nil && m.sessionInfo.State != nil {
		note = m.sessionInfo.State.GetNote(m.viewingPhase)
	}

	if note == "" {
		return []string{
			"    " + emptyStyle.Render("No notes for this phase"),
			"    " + emptyStyle.Render("Press 'n' to add a note"),
		}
	}

	// Wrap note in a box
	boxContent := noteBoxStyle.Width(m.width - 8).Render(note)
	lines := strings.Split(boxContent, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, "    "+line)
	}
	return result
}

func renderNoteInput(m Model) []string {
	// Show input with cursor
	inputText := m.noteInput
	if m.noteInputPos >= 0 && m.noteInputPos <= len(inputText) {
		// Insert cursor character
		before := inputText[:m.noteInputPos]
		after := ""
		if m.noteInputPos < len(inputText) {
			after = inputText[m.noteInputPos:]
		}
		inputText = before + "│" + after
	}

	if inputText == "" {
		inputText = "│"
	}

	boxContent := inputBoxStyle.Width(m.width - 8).Render(inputText)
	lines := strings.Split(boxContent, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, "    "+line)
	}
	result = append(result, "    "+emptyStyle.Render("Enter to save, Esc to cancel"))
	return result
}

func renderOverviewFooter(m Model) string {
	var help string
	if m.editingNote {
		help = "Enter save   Esc cancel"
	} else {
		help = "Tab diff   ◀ ▶ phases   n note   q quit"
	}
	return footerStyle.Width(m.width).Render(help)
}
