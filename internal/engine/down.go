package engine

import (
	"context"

	"github.com/ijcd/sesh/internal/spec"
)

func (e *Engine) Down(ctx context.Context, p *spec.Project) error {
	if err := RunHooks(ctx, "on_project_stop", p.Hooks.OnStop, p.Cwd); err != nil {
		return err
	}
	d, err := e.driverFor(p.Driver)
	if err != nil {
		return err
	}
	return d.Down(ctx, p.Name)
}
