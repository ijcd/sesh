package kitty

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/spec"
)

type Driver struct {
	r Runner
}

type kittyOSWindow struct {
	IsFocused bool       `json:"is_focused"`
	Tabs      []kittyTab `json:"tabs"`
}
type kittyTab struct {
	Title   string        `json:"title"`
	Windows []kittyWindow `json:"windows"`
}
type kittyWindow struct {
	IsFocused           bool                  `json:"is_focused"`
	Cwd                 string                `json:"cwd"`
	ForegroundProcesses []kittyForegroundProc `json:"foreground_processes"`
}
type kittyForegroundProc struct {
	Cmdline []string `json:"cmdline"`
}

func New() *Driver {
	return &Driver{}
}

func newWith(r Runner) *Driver { return &Driver{r: r} }

func (d *Driver) Name() string { return "kitty" }

// runner returns the Runner to use, lazily building an execRunner with
// the current KITTY_LISTEN_ON socket if no runner was injected.
func (d *Driver) runner() (Runner, error) {
	if d.r != nil {
		// For test runners, also let them know the env-detected socket if
		// they care; tests typically don't need this and pre-set things.
		if er, ok := d.r.(*execRunner); ok {
			er.SetSocket(detectSocket())
		}
		return d.r, nil
	}
	er, err := NewExecRunner()
	if err != nil {
		return nil, err
	}
	er.SetSocket(detectSocket())
	return er, nil
}

func detectSocket() string {
	return strings.TrimSpace(os.Getenv("KITTY_LISTEN_ON"))
}

func (d *Driver) Up(ctx context.Context, p *spec.Project) error {
	if detectSocket() == "" {
		return fmt.Errorf("kitty driver: KITTY_LISTEN_ON unset (run inside kitty or use --launch)")
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		return err
	}
	r, err := d.runner()
	if err != nil {
		return err
	}
	for _, line := range cmds {
		args, err := splitKittenCommand(line)
		if err != nil {
			return err
		}
		if err := r.Run(ctx, args...); err != nil {
			return fmt.Errorf("kitten %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

func (d *Driver) DryRun(p *spec.Project) ([]string, error) {
	return BuildCommands(p)
}

// splitKittenCommand parses a "kitten ..." string back into argv,
// honoring the same single-quoted format BuildCommands emits.
func splitKittenCommand(line string) ([]string, error) {
	if !strings.HasPrefix(line, "kitten ") {
		return nil, fmt.Errorf("not a kitten command: %q", line)
	}
	s := line[len("kitten "):]
	var args []string
	var buf strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' && !inQuote:
			inQuote = true
		case c == '\'' && inQuote:
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

// Compile-time assertion the Driver satisfies the interface.
var _ drivers.Driver = (*Driver)(nil)

func (d *Driver) Down(ctx context.Context, name string) error {
	r, err := d.runner()
	if err != nil {
		return err
	}
	return r.Run(ctx, "close-tab", "--match", MatchTabTitlePrefix(name))
}

func (d *Driver) Status(ctx context.Context, name string) (drivers.Status, error) {
	r, err := d.runner()
	if err != nil {
		return drivers.StatusUnknown, err
	}
	out, err := r.RunCapture(ctx, "ls")
	if err != nil {
		return drivers.StatusUnknown, fmt.Errorf("kitten ls: %w", err)
	}
	var wins []kittyOSWindow
	if err := json.Unmarshal([]byte(out), &wins); err != nil {
		return drivers.StatusUnknown, fmt.Errorf("parse kitten ls: %w", err)
	}
	prefix := name + ":"
	for _, w := range wins {
		for _, t := range w.Tabs {
			if strings.HasPrefix(t.Title, prefix) {
				return drivers.StatusExists, nil
			}
		}
	}
	return drivers.StatusNotExists, nil
}

func (d *Driver) Capture(ctx context.Context) (*spec.Project, error) {
	return nil, nil
}

func (d *Driver) Validate(p *spec.Project) []error {
	var errs []error
	for i, t := range p.Tabs {
		if strings.Contains(t.Title, ":") {
			errs = append(errs, fmt.Errorf("tabs[%d].title: must not contain ':' (sesh uses '<project>:<tab>' tagging)", i))
		}
		if err := ValidateLayout(t.Layout); err != nil {
			errs = append(errs, fmt.Errorf("tabs[%d].layout: %w", i, err))
		}
	}
	if _, err := exec.LookPath("kitten"); err != nil {
		errs = append(errs, fmt.Errorf("kitty driver: %w", err))
	}
	return errs
}

func (d *Driver) AttachCommand(p *spec.Project) (string, error) {
	return "", fmt.Errorf("kitty: AttachCommand not supported (kitty has no detached sessions)")
}
