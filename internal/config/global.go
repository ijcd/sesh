package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// Global holds defaults applied to projects when their corresponding
// scalar/map fields are unset. Lists are NOT allowed at this level —
// shared lists belong in templates.
type Global struct {
	Driver string            `yaml:"driver,omitempty"`
	Attach *bool             `yaml:"attach,omitempty"`
	Vars   map[string]string `yaml:"vars,omitempty"`
}

// allowedGlobalKeys defines which top-level keys are permitted in config.yml.
var allowedGlobalKeys = map[string]bool{
	"driver": true,
	"attach": true,
	"vars":   true,
}

// listTypedKeys are project-level keys that are list-typed and therefore
// rejected at the global level (they belong in templates).
var listTypedKeys = []string{"hooks", "pre_window", "tabs", "panes", "include"}

// LoadGlobal reads path (typically ~/.config/sesh/config.yml). Missing file → empty Global.
func LoadGlobal(path string) (*Global, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Global{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// First decode into a generic map to check for unknown / list-typed keys.
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// Check list-typed keys first for a clearer error message.
	for _, lk := range listTypedKeys {
		if _, ok := raw[lk]; ok {
			return nil, fmt.Errorf("%s: %q is a list-typed field; lists belong in templates, not global config", path, lk)
		}
	}
	for k := range raw {
		if !allowedGlobalKeys[k] {
			return nil, fmt.Errorf("%s: unknown key %q (allowed: driver, attach, vars)", path, k)
		}
	}

	var g Global
	if err := yaml.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &g, nil
}

// GlobalDefaultPath returns $XDG_CONFIG_HOME/sesh/config.yml, or ~/.config/sesh/config.yml.
func GlobalDefaultPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "sesh", "config.yml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "sesh", "config.yml"), nil
}
