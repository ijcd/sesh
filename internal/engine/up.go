package engine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/plugins"
	"github.com/ijcd/sesh/internal/spec"
)

// dispatchCrossDriverTabs splits each tab whose driver differs from the project
// driver and has panes. The inner tabs are dispatched to their child driver
// (creating that driver's container) and the outer project is mutated so each
// such tab becomes a leaf cmd that attaches to the inner container.
//
// Returns the modified project (a deep-ish copy of p) and any error.
func (e *Engine) dispatchCrossDriverTabs(ctx context.Context, p *spec.Project) (*spec.Project, error) {
	out := *p
	out.Tabs = make([]spec.Tab, len(p.Tabs))
	copy(out.Tabs, p.Tabs)

	for i := range out.Tabs {
		t := &out.Tabs[i]
		childDrv := t.Driver
		if childDrv == "" || childDrv == p.Driver || len(t.Panes) == 0 {
			continue
		}
		// Cross-driver pair: dispatch inner first.
		cd, err := e.driverFor(childDrv)
		if err != nil {
			return nil, err
		}
		innerName := slugInnerName(p.Name, t.Title)
		innerTab := *t
		innerTab.Driver = "" // child handles its own driver default
		inner := &spec.Project{
			Name: innerName, Driver: childDrv, Cwd: t.Cwd,
			Tabs: []spec.Tab{innerTab},
		}
		if inner.Cwd == "" {
			inner.Cwd = p.Cwd
		}
		if err := cd.Up(ctx, inner); err != nil {
			return nil, fmt.Errorf("cross-driver up for tab %q: %w", t.Title, err)
		}
		attachCmd, err := cd.AttachCommand(inner)
		if err != nil {
			return nil, fmt.Errorf("cross-driver AttachCommand for tab %q: %w", t.Title, err)
		}
		// Replace the outer tab with a leaf cmd that attaches.
		t.Cmd = attachCmd
		t.Panes = nil
		t.Driver = ""
	}
	return &out, nil
}

func slugInnerName(project, tab string) string {
	return project + "-" + tab
}

// Up brings the project online. force=true causes Down+Up if a session exists.
func (e *Engine) Up(ctx context.Context, p *spec.Project, force bool) error {
	if err := CheckContainment(p); err != nil {
		return err
	}

	if err := RunHooks(ctx, "pre", p.Hooks.Pre, p.Cwd); err != nil {
		return err
	}

	pp, err := e.dispatchCrossDriverTabs(ctx, p)
	if err != nil {
		return err
	}

	d, err := e.driverFor(pp.Driver)
	if err != nil {
		return err
	}

	pp = applyPreWindow(pp)

	status, err := d.Status(ctx, pp.Name)
	if err != nil {
		return fmt.Errorf("driver.Status: %w", err)
	}

	type upAction int
	const (
		actionAttachExisting upAction = iota
		actionRecreate
		actionFreshUp
	)

	action := actionFreshUp
	switch {
	case status == drivers.StatusExists && !force:
		action = actionAttachExisting
	case status == drivers.StatusExists && force:
		action = actionRecreate
	}

	switch action {
	case actionAttachExisting:
		// Attach silently — session already exists; driver handles attach in its Up happy-path.
	case actionRecreate:
		if err := d.Down(ctx, pp.Name); err != nil {
			return fmt.Errorf("force down: %w", err)
		}
		if err := d.Up(ctx, pp); err != nil {
			return err
		}
	case actionFreshUp:
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
	if err := runPluginsUp(ctx, e, pp); err != nil {
		return err
	}
	return nil
}

// runPluginsUp iterates the project's Apps in declared order, decodes each
// entry via Plugin.New, and calls Instance.Up. Dispatch order is the user's
// Apps list — registry order is irrelevant to dispatch.
//
// Failure semantics: if App.Optional is true, a New/Up error is logged to
// stderr and iteration continues. Otherwise the error aborts with no further
// plugins run (partial state stays per project policy).
func runPluginsUp(ctx context.Context, e *Engine, p *spec.Project) error {
	env := plugins.ProjectEnv{Name: p.Name, Cwd: p.Cwd}
	for i, app := range p.Apps {
		plg, ok := e.registry.Get(app.Plugin)
		if !ok {
			if app.Optional {
				fmt.Fprintf(os.Stderr, "engine: plugin %q not registered (optional, continuing)\n", app.Plugin)
				continue
			}
			return fmt.Errorf("apps[%d]: plugin %q not registered", i, app.Plugin)
		}
		raw := app.Raw
		if raw == nil {
			raw = plugins.NewRawConfig(nil)
		}
		inst, err := plg.New(env, raw)
		if err != nil {
			if app.Optional {
				fmt.Fprintf(os.Stderr, "engine: plugin %q: New failed (optional, continuing): %v\n", app.Plugin, err)
				continue
			}
			return fmt.Errorf("apps[%d] %s: New failed: %w", i, app.Key(), err)
		}
		if err := inst.Up(ctx); err != nil {
			if app.Optional {
				fmt.Fprintf(os.Stderr, "engine: plugin %q: Up failed (optional, continuing): %v\n", app.Plugin, err)
				continue
			}
			return fmt.Errorf("apps[%d] %s: Up failed: %w", i, app.Key(), err)
		}
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
