package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Critic-specific styles
	workerMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8be9fd")). // Cyan for worker
				Bold(true)

	criticResponseStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ff79c6")). // Pink for critic
				Bold(true)

	messageBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272a4")).
			Padding(0, 1)

	criticLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ff79c6")).
				Bold(true)

	workerLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8be9fd")).
				Bold(true)
)

func renderCriticView(m Model) string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	header := renderCriticHeader(m)
	b.WriteString(header)
	b.WriteString("\n")

	// Content
	contentHeight := m.height - 3
	content := renderCriticContent(m, contentHeight)
	b.WriteString(content)

	// Footer
	footer := renderCriticFooter(m)
	b.WriteString(footer)

	return b.String()
}

func renderCriticHeader(m Model) string {
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

	// Exchange position
	var position string
	if len(m.criticEntries) > 0 {
		position = fmt.Sprintf("%d/%d", m.criticIndex+1, len(m.criticEntries))
	} else {
		position = "0/0"
	}

	header := fmt.Sprintf("CRITIC │ %s │ %s │ Exchange %s", sessionName, phaseName, position)
	return headerStyle.Width(m.width).Render(header)
}

func renderCriticContent(m Model, height int) string {
	if len(m.criticEntries) == 0 {
		msg := emptyStyle.Render("No critic exchanges yet")
		hint := emptyStyle.Render("Use: piledriver critique <session> \"<context>\"")
		padding := (height - 2) / 2
		return strings.Repeat("\n", padding) + msg + "\n" + hint + strings.Repeat("\n", height-padding-2)
	}

	entry := m.criticEntries[m.criticIndex]

	var lines []string

	// Timestamp and phase
	lines = append(lines, "")
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	lines = append(lines, fmt.Sprintf("  %s │ Phase: %s", timestamp, entry.Phase))
	lines = append(lines, "")

	// Worker message
	lines = append(lines, "  "+workerLabelStyle.Render("Worker Message:"))
	lines = append(lines, "")
	workerLines := wrapText(entry.WorkerMessage, m.width-8)
	for _, line := range workerLines {
		lines = append(lines, "    "+line)
	}
	lines = append(lines, "")

	// Separator
	lines = append(lines, "  "+strings.Repeat("─", m.width-6))
	lines = append(lines, "")

	// Critic response
	lines = append(lines, "  "+criticLabelStyle.Render("Critic Response:"))
	lines = append(lines, "")
	criticLines := wrapText(entry.CriticResponse, m.width-8)
	for _, line := range criticLines {
		lines = append(lines, "    "+line)
	}
	lines = append(lines, "")

	// Apply scroll offset
	totalLines := len(lines)
	maxOffset := totalLines - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.criticScrollOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + height
	if end > totalLines {
		end = totalLines
	}

	visibleLines := lines[offset:end]

	// Pad to fill height
	for len(visibleLines) < height {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n") + "\n"
}

func renderCriticFooter(m Model) string {
	help := "Tab overview   ◀ ▶ exchanges   ▲ ▼ scroll   L newest   q quit"
	return footerStyle.Width(m.width).Render(help)
}

// wrapText wraps text to fit within a given width
func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}

	var lines []string
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) <= width {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		lines = append(lines, currentLine)
	}

	return lines
}
