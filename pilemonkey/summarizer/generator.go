package summarizer

import (
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeSummaryGenerator calls `claude -p --model haiku` to generate summaries.
type ClaudeSummaryGenerator struct{}

func (g *ClaudeSummaryGenerator) Summarize(prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", "--model", "haiku")
	cmd.Dir = "/tmp" // Run from neutral directory to avoid project context
	cmd.Stdin = strings.NewReader(prompt)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to call claude: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
