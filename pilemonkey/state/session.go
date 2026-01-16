package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

const PiledriverDir = ".piledriver"

// SessionFiles tracks which standard files exist in a session
type SessionFiles struct {
	Boundary    bool
	Assumptions bool
	ModelTLA    bool
	ModelCfg    bool
	Probe       bool
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

// FindPiledriverDir searches upward from dir for .piledriver directory
func FindPiledriverDir(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		pdDir := filepath.Join(dir, PiledriverDir)
		if info, err := os.Stat(pdDir); err == nil && info.IsDir() {
			return pdDir, nil
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

// GetSessionFiles checks which standard files exist in a session
func GetSessionFiles(sessionDir string) SessionFiles {
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(sessionDir, name))
		return err == nil
	}

	return SessionFiles{
		Boundary:    exists("boundary.md"),
		Assumptions: exists("assumptions.md"),
		ModelTLA:    exists("model.tla"),
		ModelCfg:    exists("model.cfg"),
		Probe:       exists("probe.md"),
	}
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
