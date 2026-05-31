package engine

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/ijcd/sesh/internal/plugins"
	"github.com/ijcd/sesh/internal/spec"
)

// Debug writes a rendered view of `p` to w. If commandsOnly, only the dry-run
// commands are emitted (suitable for piping to a shell).
func (e *Engine) Debug(ctx context.Context, p *spec.Project, commandsOnly bool, w io.Writer) error {
	pp := applyPreWindow(p)
	d, err := e.driverFor(p.Driver)
	if err != nil {
		return err
	}
	cmds, err := d.DryRun(pp)
	if err != nil {
		return fmt.Errorf("dry-run: %w", err)
	}

	// Plugin DryRun output is appended after driver commands so `sesh debug`
	// reflects the same Up sequence: driver first, then plugins in apps order.
	env := plugins.ProjectEnv{Name: pp.Name, Cwd: pp.Cwd}
	for i, app := range pp.Apps {
		plg, ok := e.registry.Get(app.Plugin)
		if !ok {
			continue
		}
		raw := app.Raw
		if raw == nil {
			raw = plugins.NewRawConfig(nil)
		}
		inst, perr := plg.New(env, raw)
		if perr != nil {
			cmds = append(cmds, fmt.Sprintf("# apps[%d] %s: New failed: %v", i, app.Plugin, perr))
			continue
		}
		argvs, drerr := inst.DryRun()
		if drerr != nil {
			cmds = append(cmds, fmt.Sprintf("# apps[%d] %s: DryRun failed: %v", i, app.Plugin, drerr))
			continue
		}
		cmds = append(cmds, fmt.Sprintf("# apps[%d] %s", i, app.Plugin))
		for _, line := range argvs {
			cmds = append(cmds, strings.Join(line, " "))
		}
	}

	if !commandsOnly {
		out, err := yaml.Marshal(pp)
		if err != nil {
			return fmt.Errorf("marshal spec: %w", err)
		}
		if _, err := fmt.Fprintln(w, string(out)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "--- commands ---"); err != nil {
			return err
		}
	}
	for _, c := range cmds {
		if _, err := fmt.Fprintln(w, c); err != nil {
			return err
		}
	}
	return nil
}
