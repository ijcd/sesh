package tmux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/spec"
)

// Runner is the seam for shelling out to tmux. Production uses execRunner
// (which fork/exec's `tmux ...`); tests substitute a fake.
type Runner interface {
	Run(ctx context.Context, args ...string) error
	RunCapture(ctx context.Context, args ...string) (string, error)
}

type execRunner struct {
	socketArg string // e.g., "sesh-test" → prefixes "-L sesh-test" to every tmux invocation
}

func (r *execRunner) tmuxArgs(args []string) []string {
	if r.socketArg == "" {
		return args
	}
	return append([]string{"-L", r.socketArg}, args...)
}

func (r *execRunner) Run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "tmux", r.tmuxArgs(args)...)
	cmd.Stdout = nil // discard; tmux Run commands are silent on success
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *execRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", r.tmuxArgs(args)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return out.String(), err
}

// Driver implements drivers.Driver for tmux.
type Driver struct {
	r Runner
}

// New returns a production Driver that shells out to tmux.
func New() *Driver { return &Driver{r: &execRunner{}} }

// newWith returns a Driver using the provided Runner (for testing).
func newWith(r Runner) *Driver { return &Driver{r: r} }

// WithSocket configures the driver to invoke `tmux -L <socket>` for all
// subsequent calls. Used by integration tests to isolate from the user's
// tmux server. Returns the same driver for chaining.
func (d *Driver) WithSocket(socket string) *Driver {
	if er, ok := d.r.(*execRunner); ok {
		er.socketArg = socket
	}
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
	for _, line := range cmds {
		// line begins with "tmux "; strip and split into args.
		args, err := splitTmuxCommand(line)
		if err != nil {
			return err
		}
		if err := d.r.Run(ctx, args...); err != nil {
			return fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
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
	return BuildCommands(p)
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

// splitTmuxCommand parses a `tmux ...` line back into argv. We use single-quoted
// values throughout BuildCommands, so a simple state-machine works.
func splitTmuxCommand(line string) ([]string, error) {
	if !strings.HasPrefix(line, "tmux ") {
		return nil, fmt.Errorf("not a tmux command: %q", line)
	}
	s := line[len("tmux "):]
	var args []string
	var buf strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' && !inQuote:
			inQuote = true
		case c == '\'' && inQuote:
			// handle '\'' escape
			if i+3 < len(s) && s[i:i+4] == `'\''` {
				buf.WriteByte('\'')
				i += 3
				continue
			}
			inQuote = false
		case c == ' ' && !inQuote:
			if buf.Len() > 0 {
				args = append(args, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteByte(c)
		}
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote in: %q", line)
	}
	return args, nil
}
