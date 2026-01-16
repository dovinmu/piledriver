package differ

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/dovinmu/piledriver/pilemonkey/store"
)

var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// ComputeDiff computes the diff between old and new content using git diff
func ComputeDiff(oldContent, newContent []byte) ([]store.DiffHunk, error) {
	// Create temp files for git diff
	oldFile, err := os.CreateTemp("", "pilemonkey-old-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(oldFile.Name())
	defer oldFile.Close()

	newFile, err := os.CreateTemp("", "pilemonkey-new-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(newFile.Name())
	defer newFile.Close()

	if oldContent != nil {
		if _, err := oldFile.Write(oldContent); err != nil {
			return nil, err
		}
	}
	if _, err := newFile.Write(newContent); err != nil {
		return nil, err
	}

	// Close files before git reads them
	oldFile.Close()
	newFile.Close()

	// Run git diff --no-index
	cmd := exec.Command("git", "diff", "--no-index", "-U3", "--no-color", oldFile.Name(), newFile.Name())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// git diff returns 1 if there are differences, which is expected
	err = cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means files differ (normal)
			// Exit code > 1 means error
			if exitErr.ExitCode() > 1 {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return parseDiff(stdout.String())
}

// IsBinary checks if content appears to be binary
func IsBinary(content []byte) bool {
	// Check for null bytes in first 8000 bytes
	checkLen := len(content)
	if checkLen > 8000 {
		checkLen = 8000
	}
	return bytes.Contains(content[:checkLen], []byte{0})
}

// parseDiff parses git diff output into structured hunks
func parseDiff(diffOutput string) ([]store.DiffHunk, error) {
	var hunks []store.DiffHunk
	lines := strings.Split(diffOutput, "\n")

	var currentHunk *store.DiffHunk
	oldLine, newLine := 0, 0

	for _, line := range lines {
		// Skip diff headers
		if strings.HasPrefix(line, "diff ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}

		// Parse hunk header
		if matches := hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}

			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}

			currentHunk = &store.DiffHunk{
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
				Lines:    []store.DiffLine{},
			}
			oldLine = oldStart
			newLine = newStart
			continue
		}

		if currentHunk == nil {
			continue
		}

		// Parse diff lines
		if len(line) == 0 {
			// Empty context line
			currentHunk.Lines = append(currentHunk.Lines, store.DiffLine{
				Type:    store.LineContext,
				Content: "",
				OldLine: oldLine,
				NewLine: newLine,
			})
			oldLine++
			newLine++
		} else if line[0] == '+' {
			currentHunk.Lines = append(currentHunk.Lines, store.DiffLine{
				Type:    store.LineAdded,
				Content: line[1:],
				OldLine: 0,
				NewLine: newLine,
			})
			newLine++
		} else if line[0] == '-' {
			currentHunk.Lines = append(currentHunk.Lines, store.DiffLine{
				Type:    store.LineRemoved,
				Content: line[1:],
				OldLine: oldLine,
				NewLine: 0,
			})
			oldLine++
		} else if line[0] == ' ' {
			currentHunk.Lines = append(currentHunk.Lines, store.DiffLine{
				Type:    store.LineContext,
				Content: line[1:],
				OldLine: oldLine,
				NewLine: newLine,
			})
			oldLine++
			newLine++
		} else if line[0] == '\\' {
			// "\ No newline at end of file" - skip
			continue
		}
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks, nil
}
