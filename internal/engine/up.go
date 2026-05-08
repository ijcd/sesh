package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/spec"
)

// Up brings the project online. force=true causes Down+Up if a session exists.
func (e *Engine) Up(ctx context.Context, p *spec.Project, force bool) error {
	if err := CheckContainment(p); err != nil {
		return err
	}

	if err := RunHooks(ctx, "pre", p.Hooks.Pre, p.Cwd); err != nil {
		return err
	}

	d, err := e.driverFor(p.Driver)
	if err != nil {
		return err
	}

	pp := applyPreWindow(p)

	status, err := d.Status(ctx, pp.Name)
	if err != nil {
		return fmt.Errorf("driver.Status: %w", err)
	}

	switch {
	case status == drivers.StatusExists && !force:
		// Attach silently — driver dispatches the actual attach in Up's
		// happy-path; here we skip Up because the session is already there.
	case status == drivers.StatusExists && force:
		if err := d.Down(ctx, pp.Name); err != nil {
			return fmt.Errorf("force down: %w", err)
		}
		fallthrough
	default:
		if err := d.Up(ctx, pp); err != nil {
			return err
		}
	}

	if err := RunHooks(ctx, "post", pp.Hooks.Post, pp.Cwd); err != nil {
		return err
	}
	if err := RunHooks(ctx, "on_project_start", pp.Hooks.OnStart, pp.Cwd); err != nil {
		return err
	}
	return nil
}

// applyPreWindow returns a copy of p with project-level + tab-level pre_window
// commands prepended to each pane's Cmd via " && ".
func applyPreWindow(p *spec.Project) *spec.Project {
	out := *p // shallow copy
	out.Tabs = make([]spec.Tab, len(p.Tabs))
	copy(out.Tabs, p.Tabs)
	for ti := range out.Tabs {
		prefix := append([]string{}, p.PreWindow...)
		prefix = append(prefix, out.Tabs[ti].PreWindow...)
		if len(prefix) == 0 {
			continue
		}
		out.Tabs[ti].Panes = make([]spec.Pane, len(p.Tabs[ti].Panes))
		copy(out.Tabs[ti].Panes, p.Tabs[ti].Panes)
		for pi := range out.Tabs[ti].Panes {
			existing := out.Tabs[ti].Panes[pi].Cmd
			if existing == "" {
				continue
			}
			out.Tabs[ti].Panes[pi].Cmd = strings.Join(prefix, " && ") + " && " + existing
		}
	}
	return &out
}
