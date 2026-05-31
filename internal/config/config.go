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
	resolved, err := ResolveInclude(raw, path)
	if err != nil {
		return nil, err
	}
	// Apply global defaults
	gPath, err := GlobalDefaultPath()
	if err != nil {
		return nil, err
	}
	g, err := LoadGlobal(gPath)
	if err != nil {
		return nil, err
	}
	applyGlobalDefaults(resolved, g)

	if err := ExpandVars(resolved, envOverride); err != nil {
		return nil, err
	}
	if errs := Validate(resolved, registeredDrivers); len(errs) > 0 {
		return nil, fmt.Errorf("validation failed for %s: %w", path, errors.Join(errs...))
	}
	return resolved, nil
}

// applyGlobalDefaults fills empty scalar/map fields on p with values from g.
// Lists are never touched (per design: lists belong in templates).
//
// Driver fallback order: project → global → hardcoded "tmux". Keeping the
// hardcoded fallback here lets Validate stay pure (read-only).
func applyGlobalDefaults(p *spec.Project, g *Global) {
	if p.Driver == "" {
		p.Driver = g.Driver
	}
	if p.Driver == "" {
		p.Driver = "tmux"
	}
	if p.Attach == nil && g.Attach != nil {
		p.Attach = g.Attach
	}
	// Vars: deep-merge — global's vars are added only when project doesn't have the same key.
	if len(g.Vars) > 0 {
		if p.Vars == nil {
			p.Vars = map[string]string{}
		}
		for k, v := range g.Vars {
			if _, present := p.Vars[k]; !present {
				p.Vars[k] = v
			}
		}
	}
}
