package tmux

import (
	"bytes"
	"context"
	"fmt"
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

type execRunner struct{}

func (execRunner) Run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	cmd.Stdout = nil // discard
	cmd.Stderr = nil
	return cmd.Run()
}

func (execRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

// Driver implements drivers.Driver for tmux.
type Driver struct {
	r Runner
}

// New returns a production Driver that shells out to tmux.
func New() *Driver { return &Driver{r: execRunner{}} }

// newWith returns a Driver using the provided Runner (for testing).
func newWith(r Runner) *Driver { return &Driver{r: r} }

func (d *Driver) Name() string { return "tmux" }

func (d *Driver) Up(ctx context.Context, p *spec.Project) error {
	cmds, err := BuildCommands(p)
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
	return d.r.Run(ctx, "kill-session", "-t", slug(name))
}

func (d *Driver) Status(ctx context.Context, name string) (drivers.Status, error) {
	_, err := d.r.RunCapture(ctx, "has-session", "-t", slug(name))
	if err == nil {
		return drivers.StatusExists, nil
	}
	return drivers.StatusNotExists, nil
}

func (d *Driver) DryRun(p *spec.Project) ([]string, error) {
	return BuildCommands(p)
}

// Capture is implemented in Task 16; stub satisfies drivers.Driver interface.
func (d *Driver) Capture(ctx context.Context) (*spec.Project, error) {
	return nil, nil
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
