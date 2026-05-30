package emacs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/parser"
	"github.com/ijcd/sesh/internal/plugins"
)

// rawFromYAML builds a plugins.RawConfig from a YAML source string.
func rawFromYAML(t *testing.T, src string) plugins.RawConfig {
	t.Helper()
	if src == "" {
		return plugins.NewRawConfig(nil)
	}
	f, err := parser.ParseBytes([]byte(src), 0)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	return plugins.NewRawConfig(f.Docs[0].Body)
}

// fakeRunner records every Run call and returns scripted results in order.
// When the script is exhausted later calls return nil (success).
type fakeRunner struct {
	calls   [][]string
	results []error // queued return values
	lookErr error   // returned by LookPath when set
}

func (f *fakeRunner) LookPath(bin string) (string, error) {
	if f.lookErr != nil {
		return "", f.lookErr
	}
	return "/fake/" + bin, nil
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	argv := append([]string{name}, args...)
	f.calls = append(f.calls, argv)
	if len(f.results) > 0 {
		err := f.results[0]
		f.results = f.results[1:]
		return err
	}
	return nil
}

func (f *fakeRunner) joined(i int) string {
	if i >= len(f.calls) {
		return ""
	}
	return strings.Join(f.calls[i], " ")
}

func TestEmacs_PluginNameIsEmacs(t *testing.T) {
	if got := New().Name(); got != "emacs" {
		t.Errorf("Name = %q, want %q", got, "emacs")
	}
}

func TestEmacs_NewAppliesConventionDefaults(t *testing.T) {
	p := New()
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	i := inst.(*instance)
	if i.hook != "sesh-open-project" {
		t.Errorf("hook = %q, want %q", i.hook, "sesh-open-project")
	}
	if i.close != "sesh-close-project" {
		t.Errorf("close = %q, want %q", i.close, "sesh-close-project")
	}
	if len(i.files) != 0 {
		t.Errorf("files = %v, want empty", i.files)
	}
}

func TestEmacs_NewRespectsConfigOverrides(t *testing.T) {
	p := New()
	src := "hook: my-open\nclose_hook: my-close\nfiles:\n  - a\n  - b\n"
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, src))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	i := inst.(*instance)
	if i.hook != "my-open" {
		t.Errorf("hook = %q, want %q", i.hook, "my-open")
	}
	if i.close != "my-close" {
		t.Errorf("close = %q, want %q", i.close, "my-close")
	}
	if len(i.files) != 2 || i.files[0] != "a" || i.files[1] != "b" {
		t.Errorf("files = %v, want [a b]", i.files)
	}
}

func TestEmacs_UpInvokesEmacsclientWithOpenForm(t *testing.T) {
	fr := &fakeRunner{} // probe + open form both succeed
	inst := newInstanceWith(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"},
		"sesh-open-project", "sesh-close-project", nil, fr)
	if err := inst.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	// Expect at least one call ending with the open form.
	last := fr.joined(len(fr.calls) - 1)
	if !strings.Contains(last, "emacsclient") {
		t.Fatalf("last call missing emacsclient: %q", last)
	}
	if !strings.Contains(last, "(sesh-open-project \"lib\" \"/cwd\")") {
		t.Errorf("open form missing from last call: %q", last)
	}
}

func TestEmacs_UpSpawnsDaemonWhenClientErrors(t *testing.T) {
	// Heuristic: Up probes via `emacsclient -e (emacs-version)` first. If that
	// fails (no daemon), spawn `emacs --daemon=sesh`, poll readiness via a
	// second probe, then dispatch the open form. Three runner invocations are
	// expected in order:
	//   0. probe fails (no daemon)
	//   1. daemon spawn
	//   2. probe succeeds (after spawn)
	//   3. open form dispatch
	fr := &fakeRunner{
		results: []error{
			errors.New("emacsclient: can't find socket"), // first probe: no daemon
			nil, // emacs --daemon=sesh
			nil, // second probe: ready
			nil, // open form dispatch
		},
	}
	inst := newInstanceWith(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"},
		"sesh-open-project", "sesh-close-project", nil, fr)
	if err := inst.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(fr.calls) < 3 {
		t.Fatalf("expected >=3 runner calls, got %d: %v", len(fr.calls), fr.calls)
	}
	// Locate the daemon-spawn call: must contain "emacs" and "--daemon=sesh".
	foundDaemon := false
	foundOpen := false
	for _, c := range fr.calls {
		joined := strings.Join(c, " ")
		if strings.Contains(joined, "--daemon=sesh") {
			foundDaemon = true
		}
		if strings.Contains(joined, "(sesh-open-project \"lib\" \"/cwd\")") {
			foundOpen = true
		}
	}
	if !foundDaemon {
		t.Errorf("expected --daemon=sesh spawn in calls: %v", fr.calls)
	}
	if !foundOpen {
		t.Errorf("expected open form dispatch in calls: %v", fr.calls)
	}
}

func TestEmacs_DownInvokesEmacsclientWithCloseForm(t *testing.T) {
	fr := &fakeRunner{}
	inst := newInstanceWith(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"},
		"sesh-open-project", "sesh-close-project", nil, fr)
	if err := inst.Down(context.Background()); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(fr.calls) != 1 {
		t.Fatalf("expected 1 call, got %d: %v", len(fr.calls), fr.calls)
	}
	joined := strings.Join(fr.calls[0], " ")
	if !strings.Contains(joined, "emacsclient") {
		t.Errorf("expected emacsclient: %q", joined)
	}
	if !strings.Contains(joined, "(sesh-close-project \"lib\")") {
		t.Errorf("expected close form: %q", joined)
	}
}

func TestEmacs_DownIdempotentOnClientError(t *testing.T) {
	fr := &fakeRunner{results: []error{errors.New("nope")}}
	inst := newInstanceWith(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"},
		"sesh-open-project", "sesh-close-project", nil, fr)
	if err := inst.Down(context.Background()); err != nil {
		t.Fatalf("Down returned error (best-effort contract): %v", err)
	}
}

func TestEmacs_ValidateMissingEmacsclient(t *testing.T) {
	fr := &fakeRunner{lookErr: errors.New("not found")}
	inst := newInstanceWith(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"},
		"sesh-open-project", "sesh-close-project", nil, fr)
	errs := inst.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation error")
	}
	joined := errs[0].Error()
	if !strings.Contains(joined, "emacsclient") || !strings.Contains(joined, "PATH") {
		t.Errorf("error should mention emacsclient and PATH: %v", joined)
	}
}

func TestEmacs_ValidateBadHookSymbol(t *testing.T) {
	fr := &fakeRunner{}
	inst := newInstanceWith(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"},
		"foo bar", "sesh-close-project", nil, fr)
	errs := inst.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation error")
	}
	// At least one error must mention the invalid symbol or "hook".
	hit := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "hook") || strings.Contains(e.Error(), "foo bar") || strings.Contains(e.Error(), "symbol") {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("expected error mentioning invalid hook symbol: %v", errs)
	}
}

func TestEmacs_DryRunReturnsEmacsclientArgv(t *testing.T) {
	fr := &fakeRunner{}
	inst := newInstanceWith(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"},
		"sesh-open-project", "sesh-close-project", nil, fr)
	argv, err := inst.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(argv) != 3 {
		t.Fatalf("expected argv of length 3, got %d: %v", len(argv), argv)
	}
	if argv[0] != "emacsclient" {
		t.Errorf("argv[0] = %q, want %q", argv[0], "emacsclient")
	}
	if argv[1] != "-e" {
		t.Errorf("argv[1] = %q, want %q", argv[1], "-e")
	}
	if argv[2] != `(sesh-open-project "lib" "/cwd")` {
		t.Errorf("argv[2] = %q, want %q", argv[2], `(sesh-open-project "lib" "/cwd")`)
	}
}
