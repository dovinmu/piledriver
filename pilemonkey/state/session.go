package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Directory names (prefer .piledriver hidden, support legacy piledriver/)
const PiledriverDir = ".piledriver"
const PiledriverDirLegacy = "piledriver"

// FileStatus represents the state of a session file
type FileStatus int

const (
	FileMissing  FileStatus = iota // File doesn't exist
	FileTemplate                   // File exists but only has template content
	FileFilled                     // File exists with real content
)

// SessionFiles tracks which standard files exist and their status
type SessionFiles struct {
	Reconnaissance FileStatus
	Boundary       FileStatus
	Assumptions    FileStatus
	ModelTLA       FileStatus
	CfgFiles       []string // List of .cfg files (any name)
	Probe          FileStatus
}

// ReproducerStatus represents the status of a single reproducer
type ReproducerStatus struct {
	Name       string
	HasResults bool
	Success    bool
	BaseFail   int
	FixFail    int
	Error      string // Non-empty if results.json was invalid
}

// SessionInfo contains all information about a session
type SessionInfo struct {
	Name        string
	Dir         string
	Files       SessionFiles
	Reproducers []ReproducerStatus
	State       *SessionState
}

// isPiledriverDir checks if a directory is actually a piledriver session directory.
// Distinguishes piledriver directories from git repos named 'piledriver/'.
func isPiledriverDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	// If it's a git repo, it's not a piledriver directory
	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		return false
	}

	// Check for session indicators: subdirs with state.json
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			statePath := filepath.Join(path, entry.Name(), "state.json")
			if _, err := os.Stat(statePath); err == nil {
				return true
			}
		}
	}

	// Empty piledriver dir is still valid if parent has .git
	parentGit := filepath.Join(filepath.Dir(path), ".git")
	if _, err := os.Stat(parentGit); err == nil {
		return true
	}

	return false
}

// FindPiledriverDir searches upward from dir for piledriver directory
// Supports both '.piledriver/' (preferred) and 'piledriver/' (legacy)
func FindPiledriverDir(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		// Prefer .piledriver directory
		pdDir := filepath.Join(dir, PiledriverDir)
		if isPiledriverDir(pdDir) {
			return pdDir, nil
		}

		// Fall back to legacy piledriver/
		pdDirLegacy := filepath.Join(dir, PiledriverDirLegacy)
		if isPiledriverDir(pdDirLegacy) {
			return pdDirLegacy, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			return "", nil
		}
		dir = parent
	}
}

// ListSessions returns all session names in the .piledriver directory
func ListSessions(piledriverDir string) ([]string, error) {
	entries, err := os.ReadDir(piledriverDir)
	if err != nil {
		return nil, err
	}

	var sessions []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "_tlc_out" {
			sessions = append(sessions, entry.Name())
		}
	}

	return sessions, nil
}

// FindMostRecentSession returns the most recently modified session
func FindMostRecentSession(piledriverDir string) (string, error) {
	sessions, err := ListSessions(piledriverDir)
	if err != nil {
		return "", err
	}

	if len(sessions) == 0 {
		return "", nil
	}

	type sessionWithTime struct {
		name    string
		modTime int64
	}

	var sessionsWithTime []sessionWithTime
	for _, name := range sessions {
		sessionDir := filepath.Join(piledriverDir, name)
		info, err := os.Stat(sessionDir)
		if err != nil {
			continue
		}
		sessionsWithTime = append(sessionsWithTime, sessionWithTime{
			name:    name,
			modTime: info.ModTime().UnixNano(),
		})
	}

	if len(sessionsWithTime) == 0 {
		return "", nil
	}

	// Sort by modification time, most recent first
	sort.Slice(sessionsWithTime, func(i, j int) bool {
		return sessionsWithTime[i].modTime > sessionsWithTime[j].modTime
	})

	return sessionsWithTime[0].name, nil
}

// GetSessionDir returns the full path to a session directory
func GetSessionDir(piledriverDir, sessionName string) string {
	return filepath.Join(piledriverDir, sessionName)
}

// isTemplateContent checks if file content appears to be just a template
// by looking for unfilled placeholder markers
func isTemplateContent(content string) bool {
	// Template markers that indicate unfilled content
	templateMarkers := []string{
		"<!-- TODO",
		"<!-- Describe",
		"<!-- Document",
		"<!-- Replace",
		"# TODO:",
		"TODO: Write",
		"TODO: Replace",
		"VARIABLES placeholder",  // TLA+ template marker
		"placeholder = 0",        // TLA+ template marker
	}

	for _, marker := range templateMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}

	// Also check if content is very short (likely just headers)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	nonEmptyLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines, comments, and headers
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "<!--") && !strings.HasPrefix(trimmed, "-->") {
			nonEmptyLines++
		}
	}

	// If very few non-header lines, likely still template
	return nonEmptyLines < 3
}

// getFileStatus checks if a file exists and whether it has real content
func getFileStatus(sessionDir, name string) FileStatus {
	path := filepath.Join(sessionDir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return FileMissing
	}

	if isTemplateContent(string(content)) {
		return FileTemplate
	}

	return FileFilled
}

// GetSessionFiles checks which standard files exist in a session and their status
func GetSessionFiles(sessionDir string) SessionFiles {
	files := SessionFiles{
		Reconnaissance: getFileStatus(sessionDir, "reconnaissance.md"),
		Boundary:       getFileStatus(sessionDir, "boundary.md"),
		Assumptions:    getFileStatus(sessionDir, "assumptions.md"),
		ModelTLA:       getFileStatus(sessionDir, "model.tla"),
		Probe:          getFileStatus(sessionDir, "probe.md"),
	}

	// Find all .cfg files
	entries, err := os.ReadDir(sessionDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".cfg") {
				files.CfgFiles = append(files.CfgFiles, entry.Name())
			}
		}
	}

	return files
}

// GetReproducers returns the status of all reproducers in a session
func GetReproducers(sessionDir string) ([]ReproducerStatus, error) {
	reproducersDir := filepath.Join(sessionDir, "reproducers")
	entries, err := os.ReadDir(reproducersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var reproducers []ReproducerStatus
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		status := ReproducerStatus{
			Name: entry.Name(),
		}

		resultsPath := filepath.Join(reproducersDir, entry.Name(), "results.json")
		data, err := os.ReadFile(resultsPath)
		if err != nil {
			if !os.IsNotExist(err) {
				status.Error = err.Error()
			}
			reproducers = append(reproducers, status)
			continue
		}

		status.HasResults = true

		var results struct {
			Success    bool `json:"success"`
			BaseResult struct {
				Failed int `json:"failed"`
			} `json:"base_result"`
			FixResult struct {
				Failed int `json:"failed"`
			} `json:"fix_result"`
		}

		if err := json.Unmarshal(data, &results); err != nil {
			status.Error = "invalid JSON"
			reproducers = append(reproducers, status)
			continue
		}

		status.Success = results.Success
		status.BaseFail = results.BaseResult.Failed
		status.FixFail = results.FixResult.Failed

		reproducers = append(reproducers, status)
	}

	return reproducers, nil
}

// GetSessionInfo gathers all information about a session
func GetSessionInfo(piledriverDir, sessionName string) (*SessionInfo, error) {
	sessionDir := GetSessionDir(piledriverDir, sessionName)

	// Check if session exists
	if _, err := os.Stat(sessionDir); err != nil {
		return nil, err
	}

	info := &SessionInfo{
		Name:  sessionName,
		Dir:   sessionDir,
		Files: GetSessionFiles(sessionDir),
	}

	// Load reproducers
	reproducers, err := GetReproducers(sessionDir)
	if err != nil {
		return nil, err
	}
	info.Reproducers = reproducers

	// Load state (may be nil if no state.json)
	state, err := LoadState(sessionDir)
	if err != nil {
		return nil, err
	}
	info.State = state

	return info, nil
}

// ReproducerSummary returns counts of reproducers by status
func (info *SessionInfo) ReproducerSummary() (total, passing, failing, notRun int) {
	for _, r := range info.Reproducers {
		total++
		if !r.HasResults {
			notRun++
		} else if r.Success {
			passing++
		} else {
			failing++
		}
	}
	return
}
