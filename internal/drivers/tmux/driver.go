package tmux

import (
	"context"
	"fmt"
	"strings"

	"github.com/ijcd/sesh/internal/drivers"
	dexec "github.com/ijcd/sesh/internal/drivers/exec"
	"github.com/ijcd/sesh/internal/spec"
)

// Runner is the seam for shelling out to tmux. Production uses dexec.ExecRunner
// (which fork/exec's `tmux ...`); tests substitute a fake.
type Runner = dexec.Runner

// Driver implements drivers.Driver for tmux.
type Driver struct {
	r      Runner
	socket string // non-empty when WithSocket has been called
}

// New returns a production Driver that shells out to tmux.
func New() *Driver { return &Driver{r: tmuxRunner("")} }

// newWith returns a Driver using the provided Runner (for testing).
func newWith(r Runner) *Driver { return &Driver{r: r} }

// tmuxRunner builds an ExecRunner for tmux with optional -L socket prefix.
func tmuxRunner(socket string) Runner {
	prefix := []string{}
	if socket != "" {
		prefix = []string{"-L", socket}
	}
	return dexec.NewExecRunner("tmux", prefix)
}

// WithSocket configures the driver to invoke `tmux -L <socket>` for all
// subsequent calls. Used by integration tests to isolate from the user's
// tmux server. Returns the same driver for chaining.
func (d *Driver) WithSocket(socket string) *Driver {
	d.socket = socket
	d.r = tmuxRunner(socket)
	return d
}

func (d *Driver) Name() string { return "tmux" }

func (d *Driver) Up(ctx context.Context, p *spec.Project) error {
	pbi := queryIntOption(ctx, d.r, "pane-base-index", 0)
	bi := queryIntOption(ctx, d.r, "base-index", 0)
	cmds, err := BuildCommandsWithOpts(p, BuildOpts{PaneBaseIndex: pbi, BaseIndex: bi})
	if err != nil {
		return err
	}
	for _, argv := range cmds {
		if err := d.r.Run(ctx, argv...); err != nil {
			return fmt.Errorf("tmux %s: %w", strings.Join(argv, " "), err)
		}
	}
	return nil
}

func (d *Driver) Down(ctx context.Context, name string) error {
	return d.r.Run(ctx, "kill-session", "-t", Slug(name))
}

func (d *Driver) Status(ctx context.Context, name string) (drivers.Status, error) {
	_, err := d.r.RunCapture(ctx, "has-session", "-t", Slug(name))
	if err == nil {
		return drivers.StatusExists, nil
	}
	return drivers.StatusNotExists, nil
}

func (d *Driver) DryRun(p *spec.Project) ([]string, error) {
	cmds, err := BuildCommands(p)
	if err != nil {
		return nil, err
	}
	return RenderCommands(cmds), nil
}

// Validate runs tmux-specific checks. Currently a no-op; layouts are
// passed through to tmux without sesh-side validation.
func (d *Driver) Validate(p *spec.Project) []error { return nil }

// AttachCommand returns "tmux attach -t <session>" for the project.
func (d *Driver) AttachCommand(p *spec.Project) (string, error) {
	sess := p.Session
	if sess == "" {
		sess = Slug(p.Name)
	}
	return fmt.Sprintf("tmux attach -t %s", sess), nil
}
