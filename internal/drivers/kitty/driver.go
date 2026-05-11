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
	r Runner // non-nil only when injected by newWith (tests)
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

// runner returns the Runner to use. If a runner was injected (tests), return
// it directly. Otherwise build a fresh exec runner using the current
// KITTY_LISTEN_ON socket so that --launch env changes are always picked up.
func (d *Driver) runner() (Runner, error) {
	if d.r != nil {
		return d.r, nil
	}
	bin, err := kittenPath()
	if err != nil {
		return nil, err
	}
	socket := detectSocket()
	inner := newKittenRunner(bin, socket)
	return newSocketRequiredRunner(inner, socket), nil
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
	for _, argv := range cmds {
		if err := r.Run(ctx, argv...); err != nil {
			return fmt.Errorf("kitten %s: %w", strings.Join(argv, " "), err)
		}
	}
	return nil
}

func (d *Driver) DryRun(p *spec.Project) ([]string, error) {
	cmds, err := BuildCommands(p)
	if err != nil {
		return nil, err
	}
	return RenderCommands(cmds), nil
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
