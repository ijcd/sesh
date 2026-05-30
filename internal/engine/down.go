package engine

import (
	"context"
	"fmt"
	"os"

	"github.com/ijcd/sesh/internal/plugins"
	"github.com/ijcd/sesh/internal/spec"
)

func (e *Engine) Down(ctx context.Context, p *spec.Project) error {
	if err := RunHooks(ctx, "on_project_stop", p.Hooks.OnStop, p.Cwd); err != nil {
		return err
	}
	// Plugins go down before the driver: app-side teardown should happen
	// while the host terminal is still alive.
	if err := runPluginsDown(ctx, e, p); err != nil {
		return err
	}
	d, err := e.driverFor(p.Driver)
	if err != nil {
		return err
	}
	return d.Down(ctx, p.Name)
}

// runPluginsDown walks p.Apps in REVERSE declared order, best-effort. Errors
// from Plugin.New or Instance.Down are logged to stderr and iteration
// continues — teardown does not abort on a single plugin failure.
func runPluginsDown(ctx context.Context, e *Engine, p *spec.Project) error {
	env := plugins.ProjectEnv{Name: p.Name, Cwd: p.Cwd}
	for i := len(p.Apps) - 1; i >= 0; i-- {
		app := p.Apps[i]
		plg, ok := e.registry.Get(app.Plugin)
		if !ok {
			fmt.Fprintf(os.Stderr, "engine: plugin %q not registered (down, continuing)\n", app.Plugin)
			continue
		}
		raw := app.Raw
		if raw == nil {
			raw = plugins.NewRawConfig(nil)
		}
		inst, err := plg.New(env, raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "engine: plugin %q New failed (down, continuing): %v\n", app.Plugin, err)
			continue
		}
		if err := inst.Down(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "engine: plugin %q Down failed (continuing): %v\n", app.Plugin, err)
			continue
		}
	}
	return nil
}
