package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dovinmu/piledriver/pilemonkey/store"
	"github.com/dovinmu/piledriver/pilemonkey/syntax"
)

var (
	// Colors
	liveColor    = lipgloss.Color("#ff5555")
	pausedColor  = lipgloss.Color("#6272a4")
	addedColor   = lipgloss.Color("#50fa7b")
	removedColor = lipgloss.Color("#ff5555")
	contextColor = lipgloss.Color("#6272a4")
	headerBg     = lipgloss.Color("#44475a")
	footerBg     = lipgloss.Color("#282a36")

	// Styles
	headerStyle = lipgloss.NewStyle().
			Background(headerBg).
			Foreground(lipgloss.Color("#f8f8f2")).
			Bold(true).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Background(footerBg).
			Foreground(lipgloss.Color("#6272a4")).
			Padding(0, 1)

	addedLineStyle = lipgloss.NewStyle().
			Foreground(addedColor)

	removedLineStyle = lipgloss.NewStyle().
				Foreground(removedColor)

	contextLineStyle = lipgloss.NewStyle().
				Foreground(contextColor)

	lineNumStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272a4")).
			Width(5).
			Align(lipgloss.Right)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272a4")).
			Italic(true)

	// Syntax highlighter instance
	highlighter = syntax.New()
)

func renderView(m Model) string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Dispatch based on view mode
	if m.viewMode == ViewModeOverview {
		return renderOverview(m)
	}
	return renderDiffView(m)
}

func renderDiffView(m Model) string {
	var b strings.Builder

	// Header
	header := renderDiffHeader(m)
	b.WriteString(header)
	b.WriteString("\n")

	// Content area
	contentHeight := m.height - 3 // header + footer + newlines
	content := renderDiffContent(m, contentHeight)
	b.WriteString(content)

	// Footer
	footer := renderDiffFooter(m)
	b.WriteString(footer)

	return b.String()
}

func renderDiffHeader(m Model) string {
	cs, ok := m.GetCurrentChangeset()

	// Live indicator
	var indicator string
	if m.liveMode {
		indicator = lipgloss.NewStyle().Foreground(liveColor).Bold(true).Render("● LIVE")
	} else {
		indicator = lipgloss.NewStyle().Foreground(pausedColor).Render("○ PAUSED")
	}

	// File info
	var fileInfo string
	if ok {
		relPath := m.RelativePath(cs.FilePath)
		position := fmt.Sprintf("%d/%d", m.currentIndex+1, m.store.Size())
		timestamp := cs.Timestamp.Format("15:04:05")
		fileInfo = fmt.Sprintf("%s │ %s │ %s", relPath, position, timestamp)
	} else {
		fileInfo = "No changes yet"
	}

	header := fmt.Sprintf("%s │ %s", indicator, fileInfo)
	return headerStyle.Width(m.width).Render(header)
}

func renderDiffContent(m Model, height int) string {
	cs, ok := m.GetCurrentChangeset()
	if !ok {
		msg := emptyStyle.Render("Waiting for file changes...")
		// Center vertically
		padding := (height - 1) / 2
		return strings.Repeat("\n", padding) + msg + strings.Repeat("\n", height-padding-1)
	}

	// Build all diff lines
	var lines []string

	if cs.IsBinary {
		lines = append(lines, emptyStyle.Render("Binary file changed"))
	} else if cs.IsDeleted {
		lines = append(lines, removedLineStyle.Render("File deleted"))
	} else if cs.IsNew && len(cs.Hunks) == 0 {
		// New empty file
		lines = append(lines, addedLineStyle.Render("New empty file created"))
	} else if len(cs.Hunks) == 0 {
		lines = append(lines, emptyStyle.Render("No changes"))
	} else {
		for i, hunk := range cs.Hunks {
			if i > 0 {
				lines = append(lines, contextLineStyle.Render("───────────────────────────────────"))
			}

			// Hunk header
			hunkHeader := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
				hunk.OldStart, hunk.OldCount,
				hunk.NewStart, hunk.NewCount)
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#8be9fd")).Render(hunkHeader))

			for _, line := range hunk.Lines {
				lineStr := renderDiffLine(line, m.width, cs.FilePath)
				lines = append(lines, lineStr)
			}
		}
	}

	// Apply scroll offset
	totalLines := len(lines)

	// Clamp scroll offset
	maxOffset := totalLines - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}

	// Get visible lines
	start := m.scrollOffset
	end := start + height
	if end > totalLines {
		end = totalLines
	}

	visibleLines := lines[start:end]

	// Pad to fill height
	for len(visibleLines) < height {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n") + "\n"
}

func renderDiffLine(line store.DiffLine, width int, filename string) string {
	var prefix string
	var style lipgloss.Style
	var lineNum string

	switch line.Type {
	case store.LineAdded:
		prefix = "+"
		style = addedLineStyle
		lineNum = lineNumStyle.Render(fmt.Sprintf("%d", line.NewLine))
	case store.LineRemoved:
		prefix = "-"
		style = removedLineStyle
		lineNum = lineNumStyle.Render(fmt.Sprintf("%d", line.OldLine))
	default:
		prefix = " "
		style = contextLineStyle
		lineNum = lineNumStyle.Render(fmt.Sprintf("%d", line.NewLine))
	}

	content := line.Content
	// Truncate long lines
	maxContent := width - 8 // line num + prefix + padding
	if maxContent < 10 {
		maxContent = 10
	}
	if len(content) > maxContent {
		content = content[:maxContent-3] + "..."
	}

	// Apply syntax highlighting for added/removed lines
	var styledContent string
	if line.Type == store.LineAdded || line.Type == store.LineRemoved {
		tokens := highlighter.Highlight(filename, content)
		styledContent = syntax.RenderLine(tokens, style)
	} else {
		styledContent = style.Render(content)
	}

	return fmt.Sprintf("%s %s %s", lineNum, style.Render(prefix), styledContent)
}

func renderDiffFooter(m Model) string {
	help := "Tab overview   ◀ ▶ changes   ▲ ▼ scroll   L live   q quit"
	return footerStyle.Width(m.width).Render(help)
}
