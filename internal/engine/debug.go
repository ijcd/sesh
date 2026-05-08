package engine

import (
	"context"
	"fmt"
	"io"

	"github.com/goccy/go-yaml"

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
