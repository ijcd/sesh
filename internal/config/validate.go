package config

import (
	"fmt"

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
func Validate(p *spec.Project, registeredDrivers []string) []error {
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
	return errs
}
