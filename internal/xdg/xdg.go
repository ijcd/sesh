// Package xdg resolves XDG base directories with a single fallback to $HOME/.config or $HOME/.local/state.
// Used by sesh in place of inline XDG resolution at every config / state callsite.
package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigHome returns $XDG_CONFIG_HOME if set, else $HOME/.config.
// Returns an error if $HOME is also unavailable (extremely rare; CI/sandbox edge cases).
func ConfigHome() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}

// StateHome returns $XDG_STATE_HOME if set, else $HOME/.local/state.
// Returns an error if $HOME is also unavailable.
func StateHome() (string, error) {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state"), nil
}
