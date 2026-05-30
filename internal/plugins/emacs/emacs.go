package emacs

// This file implements the lifecycle side of the emacs plugin: the Plugin
// factory, per-project Instance, and the emacsclient dispatch. The pure
// elisp form builders live in elisp.go — keep building separate from
// shelling out so the builders stay testable as plain string functions.
//
// Mechanism-not-policy: sesh shells out to `emacsclient -e <form>`. The form
// is a one-line call to a user-defined hook (e.g. `sesh-open-project`). All
// layout policy lives in the user's emacs config.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/ijcd/sesh/internal/plugins"
)

// emacsConfig is the plugin's per-Apps[] entry config schema. All fields are
// optional; convention defaults are applied inside Plugin.New.
type emacsConfig struct {
	Hook      string   `yaml:"hook,omitempty"`       // open hook name; default "sesh-open-project"
	CloseHook string   `yaml:"close_hook,omitempty"` // close hook name; default "sesh-close-project"
	Daemon    string   `yaml:"daemon,omitempty"`     // daemon name; default "sesh"
	Files     []string `yaml:"files,omitempty"`      // optional files passed to the hook
}

// runner is the seam for shelling out, narrowed to what the emacs plugin needs:
// LookPath for Validate, Run for Up/Down. Tests inject a fake; production uses
// execRunner backed by os/exec.
type runner interface {
	LookPath(bin string) (string, error)
	Run(ctx context.Context, name string, args ...string) error
}

// execRunner is the production runner: real PATH lookup, real os/exec calls.
// Stderr is forwarded so emacsclient diagnostics surface during `sesh up`.
type execRunner struct{}

func (execRunner) LookPath(bin string) (string, error) { return exec.LookPath(bin) }

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// plugin is the stateless factory. Each Project gets a fresh instance via New.
type plugin struct {
	r runner // non-nil only when injected by tests
}

// New constructs the emacs Plugin. The emacsclient binary is looked up
// lazily inside Validate / Up — missing binary surfaces there, not at
// construction time. Pattern mirrors internal/drivers/kitty.New().
func New() plugins.Plugin { return &plugin{} }

// Name reports the registry key.
func (p *plugin) Name() string { return "emacs" }

// New decodes cfg into emacsConfig, applies convention defaults, and returns
// a per-project Instance bound to env.
func (p *plugin) New(env plugins.ProjectEnv, cfg plugins.RawConfig) (plugins.Instance, error) {
	var c emacsConfig
	if err := cfg.Decode(&c); err != nil {
		return nil, fmt.Errorf("emacs: decode config: %w", err)
	}
	if c.Hook == "" {
		c.Hook = "sesh-open-project"
	}
	if c.CloseHook == "" {
		c.CloseHook = "sesh-close-project"
	}
	if c.Daemon == "" {
		c.Daemon = "sesh"
	}
	r := p.r
	if r == nil {
		r = execRunner{}
	}
	return &instance{
		env:    env,
		hook:   c.Hook,
		close:  c.CloseHook,
		daemon: c.Daemon,
		files:  c.Files,
		r:      r,
	}, nil
}

// instance is one configured emacs sidecar per `sesh up`.
type instance struct {
	env    plugins.ProjectEnv
	hook   string
	close  string
	daemon string
	files  []string
	r      runner
}

// newInstanceWith builds an instance with an injected runner. Test-only seam.
// Callers pass the daemon name explicitly so tests can exercise non-default
// daemon names; production callers pass "sesh" to match the convention default
// applied in Plugin.New.
func newInstanceWith(env plugins.ProjectEnv, hook, close, daemon string, files []string, r runner) *instance {
	return &instance{env: env, hook: hook, close: close, daemon: daemon, files: files, r: r}
}

// daemonWaitTimeout caps how long Up waits for a freshly spawned daemon to
// become ready (probe via emacsclient succeeds). Five seconds matches the
// kitty driver's WaitForSocket pattern at a coarser polling interval.
const daemonWaitTimeout = 5 * time.Second

// daemonPollInterval is the gap between readiness probes after spawning the
// daemon. 100 ms — fast enough that a normal-launch daemon registers within
// 1-2 polls, slow enough not to spin.
const daemonPollInterval = 100 * time.Millisecond

// Up dispatches the open hook via emacsclient. If the first call fails it
// assumes no daemon is running, spawns `emacs --daemon=<i.daemon>` (the
// configured daemon name, default `sesh`), waits for it to become ready,
// then retries. Idempotent: calling Up twice when the daemon is already up
// runs the hook twice (the hook itself decides whether to no-op on re-open).
func (i *instance) Up(ctx context.Context) error {
	form := BuildOpenForm(i.hook, i.env.Name, i.env.Cwd, i.files)
	// Probe first: cheap, unambiguous "is daemon up?" signal. The probe is a
	// no-op elisp form so a successful run proves the daemon answered.
	if err := i.r.Run(ctx, "emacsclient", "--socket-name="+i.daemon, "-e", "(emacs-version)"); err != nil {
		// Probe failed — assume no daemon. Spawn one and wait for readiness.
		if err := i.r.Run(ctx, "emacs", "--daemon="+i.daemon); err != nil {
			return fmt.Errorf("emacs: spawn daemon: %w", err)
		}
		if err := i.waitForDaemon(ctx); err != nil {
			return fmt.Errorf("emacs: daemon not ready: %w", err)
		}
	}
	if err := i.r.Run(ctx, "emacsclient", "--socket-name="+i.daemon, "-e", form); err != nil {
		return fmt.Errorf("emacs: dispatch open hook: %w", err)
	}
	return nil
}

// waitForDaemon polls emacsclient at daemonPollInterval until the probe
// succeeds, daemonWaitTimeout elapses, or ctx is cancelled.
func (i *instance) waitForDaemon(ctx context.Context) error {
	deadline := time.Now().Add(daemonWaitTimeout)
	for {
		if err := i.r.Run(ctx, "emacsclient", "--socket-name="+i.daemon, "-e", "(emacs-version)"); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s", daemonWaitTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(daemonPollInterval):
		}
	}
}

// Down dispatches the close hook, best-effort. Errors are logged to stderr
// and swallowed: the daemon may serve other projects, and `sesh down` must
// not be destructive across projects.
func (i *instance) Down(ctx context.Context) error {
	form := BuildCloseForm(i.close, i.env.Name)
	if err := i.r.Run(ctx, "emacsclient", "--socket-name="+i.daemon, "-e", form); err != nil {
		fmt.Fprintf(os.Stderr, "emacs: dispatch close hook (best-effort, continuing): %v\n", err)
	}
	return nil
}

// elispSymbol matches a permissive elisp symbol: at least one char, no space,
// no paren, no quote/backtick/comma. Conservative — real elisp allows more
// punctuation, but anything outside this set in a hook name is almost
// certainly a typo (e.g. a space) that we want to flag at Validate time.
var elispSymbol = regexp.MustCompile(`^[^\s()'"` + "`" + `,]+$`)

// Validate checks that emacsclient is on PATH and that the hook symbols look
// like valid elisp. File paths are not stat'd: the user's hook may create
// them on demand.
func (i *instance) Validate() []error {
	var errs []error
	if _, err := i.r.LookPath("emacsclient"); err != nil {
		errs = append(errs, fmt.Errorf("emacs: emacsclient not found in PATH (install emacs or use a terminal cmd: tab instead)"))
	}
	if !elispSymbol.MatchString(i.hook) {
		errs = append(errs, fmt.Errorf("emacs: hook %q is not a valid elisp symbol", i.hook))
	}
	if !elispSymbol.MatchString(i.close) {
		errs = append(errs, fmt.Errorf("emacs: close_hook %q is not a valid elisp symbol", i.close))
	}
	return errs
}

// DryRun returns the argv `sesh up` would dispatch on a daemon-already-up
// path. The daemon-spawn fallback is not previewed: DryRun is a static
// snapshot, not a state-machine trace.
func (i *instance) DryRun() ([]string, error) {
	form := BuildOpenForm(i.hook, i.env.Name, i.env.Cwd, i.files)
	return []string{"emacsclient", "--socket-name=" + i.daemon, "-e", form}, nil
}

// Compile-time assertions.
var (
	_ plugins.Plugin   = (*plugin)(nil)
	_ plugins.Instance = (*instance)(nil)
)
