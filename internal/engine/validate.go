package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/spec"
)

// Validate runs engine-level checks (containment) and driver-level checks.
// Returns nil if all pass; otherwise an aggregated error.
func (e *Engine) Validate(_ context.Context, p *spec.Project) error {
	var errs []error
	if err := CheckContainment(p); err != nil {
		errs = append(errs, err)
	}
	d, err := e.driverFor(p.Driver)
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, ve := range d.Validate(p) {
			errs = append(errs, ve)
		}
	}
	// Plugin-registry membership check (config.Validate is driver-only;
	// engine layer is where the registry lives).
	for _, ve := range config.ValidateAppsWithRegistry(p, e.registry) {
		errs = append(errs, ve)
	}
	// Tab-driver overrides also get their driver's Validate.
	seen := map[string]bool{p.Driver: true}
	for _, t := range p.Tabs {
		if t.Driver == "" || seen[t.Driver] {
			continue
		}
		seen[t.Driver] = true
		td, err := e.driverFor(t.Driver)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, ve := range td.Validate(p) {
			errs = append(errs, ve)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("validation failed: %w", errors.Join(errs...))
}
