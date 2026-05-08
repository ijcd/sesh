package config

import (
	"errors"
	"fmt"

	"github.com/ijcd/sesh/internal/spec"
)

// Load resolves a project by name from the standard config dir, applying the
// full pipeline: file → extends → merge → vars → validate.
// envOverride is for testing; pass nil for normal os.Getenv-based lookup.
func Load(name string, registeredDrivers []string, envOverride map[string]string) (*spec.Project, error) {
	path, err := ResolveProjectPath(name)
	if err != nil {
		return nil, err
	}
	return LoadFromPath(path, registeredDrivers, envOverride)
}

// LoadFromPath does the same as Load but takes an explicit file path
// (used by `sesh local` and tests).
func LoadFromPath(path string, registeredDrivers []string, envOverride map[string]string) (*spec.Project, error) {
	raw, err := LoadFile(path)
	if err != nil {
		return nil, err
	}
	resolved, err := ResolveExtends(raw, path)
	if err != nil {
		return nil, err
	}
	if err := ExpandVars(resolved, envOverride); err != nil {
		return nil, err
	}
	if errs := Validate(resolved, registeredDrivers); len(errs) > 0 {
		return nil, fmt.Errorf("validation failed for %s: %w", path, errors.Join(errs...))
	}
	return resolved, nil
}
