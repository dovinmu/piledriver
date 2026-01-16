package tui

import "github.com/charmbracelet/bubbletea"

// KeyMap defines the key bindings for the TUI
type KeyMap struct {
	Quit       []string
	PrevChange []string
	NextChange []string
	ScrollUp   []string
	ScrollDown []string
	PageUp     []string
	PageDown   []string
	Top        []string
	Bottom     []string
	Live       []string
	ToggleView []string
	AddNote    []string
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:       []string{"q", "ctrl+c"},
		PrevChange: []string{"left", "h"},
		NextChange: []string{"right", "l"},
		ScrollUp:   []string{"up", "k"},
		ScrollDown: []string{"down", "j"},
		PageUp:     []string{"pgup", "ctrl+b"},
		PageDown:   []string{"pgdown", "ctrl+f"},
		Top:        []string{"home", "g"},
		Bottom:     []string{"end", "G"},
		Live:       []string{"L"},
		ToggleView: []string{"tab"},
		AddNote:    []string{"n"},
	}
}

// matchesKey checks if a key message matches any of the given key strings
func matchesKey(msg tea.KeyMsg, keys []string) bool {
	for _, k := range keys {
		if msg.String() == k {
			return true
		}
	}
	return false
}
