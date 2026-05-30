package config

import (
	"fmt"

	"github.com/ijcd/sesh/internal/plugins"
	"github.com/ijcd/sesh/internal/spec"
)

// ValidationError carries a key path + reason. Implements error.
type ValidationError struct {
	Path   string
	Reason string
}

func (e ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Path, e.Reason) }

// Validate checks structural rules. Mutates p only to default the Driver field
// when empty. Returns all violations (caller decides how to surface them).
//
// Plugin-registry membership is not checked here — pass a non-nil registry
// to ValidateWithRegistry for that. T5 wires it from the engine.
func Validate(p *spec.Project, registeredDrivers []string) []error {
	return ValidateWithRegistry(p, registeredDrivers, nil)
}

// ValidateWithRegistry adds plugin-registry membership checks to Validate.
// Pass registry=nil to skip the check (useful pre-engine-wire and in tests).
func ValidateWithRegistry(p *spec.Project, registeredDrivers []string, registry *plugins.Registry) []error {
	var errs []error

	if p.Driver == "" {
		p.Driver = "tmux"
	}
	registered := false
	for _, d := range registeredDrivers {
		if d == p.Driver {
			registered = true
		}
	}
	if !registered {
		errs = append(errs, ValidationError{Path: "driver",
			Reason: fmt.Sprintf("driver %q not registered (v0.1: %v)", p.Driver, registeredDrivers)})
	}

	if p.Extends != "" {
		errs = append(errs, ValidationError{
			Path:   "extends",
			Reason: "extends: is no longer supported (sesh v0.3+); rename to include:",
		})
	}

	if len(p.Tabs) == 0 {
		errs = append(errs, ValidationError{Path: "tabs", Reason: "at least one tab is required"})
	}

	seenTab := map[string]bool{}
	for ti, t := range p.Tabs {
		prefix := fmt.Sprintf("tabs[%d]", ti)
		if t.Title == "" {
			errs = append(errs, ValidationError{Path: prefix + ".title", Reason: "required"})
			continue
		}
		if seenTab[t.Title] {
			errs = append(errs, ValidationError{Path: prefix + ".title",
				Reason: fmt.Sprintf("duplicate title %q", t.Title)})
		}
		seenTab[t.Title] = true

		if t.Cmd != "" && len(t.Panes) > 0 {
			errs = append(errs, ValidationError{Path: prefix,
				Reason: "cmd and panes are mutually exclusive"})
		}

		seenPane := map[string]bool{}
		for pi, pn := range t.Panes {
			ppref := fmt.Sprintf("%s.panes[%d]", prefix, pi)
			if pn.Title == "" {
				errs = append(errs, ValidationError{Path: ppref + ".title", Reason: "required"})
				continue
			}
			if pn.Cmd == "" {
				errs = append(errs, ValidationError{Path: ppref + ".cmd", Reason: "required"})
			}
			if seenPane[pn.Title] {
				errs = append(errs, ValidationError{Path: ppref + ".title",
					Reason: fmt.Sprintf("duplicate title %q", pn.Title)})
			}
			seenPane[pn.Title] = true
		}
	}

	errs = append(errs, validateApps(p, registry)...)
	return errs
}

// ValidateAppsWithRegistry runs only the apps[]-related checks. Engine uses
// this to add plugin-registry membership checks without re-running driver/tab
// validation (which engine handles via driver.Validate + CheckContainment).
func ValidateAppsWithRegistry(p *spec.Project, registry *plugins.Registry) []error {
	return validateApps(p, registry)
}

// validateApps checks structural rules for a project's apps[]:
//   - every entry has a non-empty Plugin name;
//   - explicit IDs are unique per plugin (auto-indexed IDs are already
//     unique by construction in spec.normalizeApps);
//   - if registry is non-nil, every entry's Plugin is registered.
//
// Registry can be nil (engine wires it in T5). When nil, the
// registered-plugin check is skipped.
func validateApps(p *spec.Project, registry *plugins.Registry) []error {
	var errs []error
	type pidKey struct{ plugin, id string }
	seen := map[pidKey]int{}
	for i, a := range p.Apps {
		prefix := fmt.Sprintf("apps[%d]", i)
		if a.Plugin == "" {
			errs = append(errs, ValidationError{Path: prefix + ".plugin", Reason: "required"})
			continue
		}
		if a.ID != "" {
			k := pidKey{a.Plugin, a.ID}
			if prev, ok := seen[k]; ok {
				errs = append(errs, ValidationError{Path: prefix + ".id",
					Reason: fmt.Sprintf("duplicate id %q for plugin %q (also at apps[%d])", a.ID, a.Plugin, prev)})
			} else {
				seen[k] = i
			}
		}
		if registry != nil {
			if _, ok := registry.Get(a.Plugin); !ok {
				errs = append(errs, ValidationError{Path: prefix + ".plugin",
					Reason: fmt.Sprintf("plugin %q not registered (registered: %v)", a.Plugin, registry.Names())})
			}
		}
	}
	return errs
}
