package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/plugins"
	"github.com/ijcd/sesh/internal/spec"
)

// callLog is a shared, ordered record of lifecycle events for ordering
// assertions across the fake driver/plugin/instance.
type callLog struct {
	mu      sync.Mutex
	entries []string
}

func (l *callLog) add(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, s)
}

func (l *callLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.entries))
	copy(out, l.entries)
	return out
}

// driverShim is a minimal drivers.Driver that records Up/Down to a shared log.
// Status defaults to StatusNotExists so Up follows the fresh-up path.
type driverShim struct {
	NameVal string
	Log     *callLog
	UpErr   error
	DownErr error
}

func (d *driverShim) Name() string { return d.NameVal }
func (d *driverShim) Up(ctx context.Context, p *spec.Project) error {
	d.Log.add(fmt.Sprintf("driver:up:%s", p.Name))
	return d.UpErr
}
func (d *driverShim) Down(ctx context.Context, name string) error {
	d.Log.add(fmt.Sprintf("driver:down:%s", name))
	return d.DownErr
}
func (d *driverShim) Status(ctx context.Context, name string) (drivers.Status, error) {
	return drivers.StatusNotExists, nil
}
func (d *driverShim) Capture(ctx context.Context) ([]*spec.Project, error) { return nil, nil }
func (d *driverShim) DryRun(p *spec.Project) ([]string, error)             { return nil, nil }
func (d *driverShim) AttachCommand(p *spec.Project) (string, error)        { return "", nil }
func (d *driverShim) Validate(p *spec.Project) []error                     { return nil }

// fakePlugin / fakeInstance let tests assert lifecycle order via a shared log.
type fakePlugin struct {
	name    string
	log     *callLog
	upErr   error
	downErr error
	newErr  error
}

func (p *fakePlugin) Name() string { return p.name }
func (p *fakePlugin) New(env plugins.ProjectEnv, cfg plugins.RawConfig) (plugins.Instance, error) {
	if p.newErr != nil {
		return nil, p.newErr
	}
	return &fakeInstance{
		plugin:  p.name,
		project: env.Name,
		log:     p.log,
		upErr:   p.upErr,
		downErr: p.downErr,
	}, nil
}

type fakeInstance struct {
	plugin  string
	project string
	log     *callLog
	upErr   error
	downErr error
}

func (i *fakeInstance) Up(ctx context.Context) error {
	i.log.add(fmt.Sprintf("plugin:up:%s:%s", i.plugin, i.project))
	return i.upErr
}
func (i *fakeInstance) Down(ctx context.Context) error {
	i.log.add(fmt.Sprintf("plugin:down:%s:%s", i.plugin, i.project))
	return i.downErr
}
func (i *fakeInstance) Validate() []error         { return nil }
func (i *fakeInstance) DryRun() ([]string, error) { return nil, nil }

// newRecEngine returns an engine with a driverShim registered under driverName
// plus the shared log for assertions.
func newRecEngine(driverName string) (*Engine, *callLog) {
	log := &callLog{}
	e := New()
	e.Register(&driverShim{NameVal: driverName, Log: log})
	return e, log
}

// --- tests --------------------------------------------------------------

func TestEngineUp_DriverBeforePlugins(t *testing.T) {
	e, log := newRecEngine("tmux")
	if err := e.RegisterPlugin(&fakePlugin{name: "emacs", log: log}); err != nil {
		t.Fatal(err)
	}
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
		Apps: []spec.App{spec.NewApp("emacs", "")},
	}
	if err := e.Up(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}
	got := log.snapshot()
	driverIdx, pluginIdx := -1, -1
	for i, s := range got {
		if strings.HasPrefix(s, "driver:up") && driverIdx == -1 {
			driverIdx = i
		}
		if strings.HasPrefix(s, "plugin:up") && pluginIdx == -1 {
			pluginIdx = i
		}
	}
	if driverIdx == -1 || pluginIdx == -1 {
		t.Fatalf("missing driver:up or plugin:up in %v", got)
	}
	if driverIdx > pluginIdx {
		t.Errorf("driver:up at %d should precede plugin:up at %d (log=%v)", driverIdx, pluginIdx, got)
	}
}

func TestEngineDown_PluginsBeforeDriverReverse(t *testing.T) {
	e, log := newRecEngine("tmux")
	if err := e.RegisterPlugin(&fakePlugin{name: "a", log: log}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegisterPlugin(&fakePlugin{name: "b", log: log}); err != nil {
		t.Fatal(err)
	}
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
		Apps: []spec.App{spec.NewApp("a", ""), spec.NewApp("b", "")},
	}
	if err := e.Down(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	got := log.snapshot()
	want := []string{"plugin:down:b:x", "plugin:down:a:x", "driver:down:x"}
	if len(got) != len(want) {
		t.Fatalf("len(log) = %d, want %d (log=%v)", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("log[%d] = %q, want %q (full=%v)", i, got[i], w, got)
		}
	}
}

func TestEngineUp_OptionalPluginSoftFails(t *testing.T) {
	e, log := newRecEngine("tmux")
	if err := e.RegisterPlugin(&fakePlugin{name: "bad", log: log, upErr: errors.New("boom")}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegisterPlugin(&fakePlugin{name: "good", log: log}); err != nil {
		t.Fatal(err)
	}
	bad := spec.NewApp("bad", "")
	bad.Optional = true
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
		Apps: []spec.App{bad, spec.NewApp("good", "")},
	}
	if err := e.Up(context.Background(), p, false); err != nil {
		t.Fatalf("optional plugin error should be soft-failed; got %v", err)
	}
	got := log.snapshot()
	foundGood := false
	for _, s := range got {
		if s == "plugin:up:good:x" {
			foundGood = true
		}
	}
	if !foundGood {
		t.Errorf("subsequent plugins should run after optional soft-fail; log=%v", got)
	}
}

func TestEngineUp_RequiredPluginErrorAborts(t *testing.T) {
	e, log := newRecEngine("tmux")
	if err := e.RegisterPlugin(&fakePlugin{name: "bad", log: log, upErr: errors.New("boom")}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegisterPlugin(&fakePlugin{name: "good", log: log}); err != nil {
		t.Fatal(err)
	}
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
		Apps: []spec.App{spec.NewApp("bad", ""), spec.NewApp("good", "")},
	}
	err := e.Up(context.Background(), p, false)
	if err == nil {
		t.Fatal("required plugin error should abort Up")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("want error containing 'boom', got %v", err)
	}
	got := log.snapshot()
	for _, s := range got {
		if s == "plugin:up:good:x" {
			t.Errorf("subsequent plugins should NOT run after required-plugin abort; log=%v", got)
		}
	}
}

func TestEngine_PluginsDispatchInAppsListOrder(t *testing.T) {
	e, log := newRecEngine("tmux")
	// Registration order: c, b, a (reverse of Apps order).
	if err := e.RegisterPlugin(&fakePlugin{name: "c", log: log}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegisterPlugin(&fakePlugin{name: "b", log: log}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegisterPlugin(&fakePlugin{name: "a", log: log}); err != nil {
		t.Fatal(err)
	}
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
		Apps: []spec.App{
			spec.NewApp("a", ""),
			spec.NewApp("b", ""),
			spec.NewApp("c", ""),
		},
	}
	if err := e.Up(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}
	got := log.snapshot()
	want := []string{"plugin:up:a:x", "plugin:up:b:x", "plugin:up:c:x"}
	idx := 0
	for _, s := range got {
		if idx < len(want) && s == want[idx] {
			idx++
		}
	}
	if idx != len(want) {
		t.Errorf("plugin:up order != Apps order; want subsequence %v, got log=%v", want, got)
	}
}

func TestEnginePlugins_ReturnsRegistrationOrder(t *testing.T) {
	e := New()
	if err := e.RegisterPlugin(&fakePlugin{name: "z"}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegisterPlugin(&fakePlugin{name: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegisterPlugin(&fakePlugin{name: "m"}); err != nil {
		t.Fatal(err)
	}
	got := e.Plugins()
	want := []string{"z", "a", "m"}
	if len(got) != len(want) {
		t.Fatalf("Plugins() = %v, want %v", got, want)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("Plugins()[%d] = %q, want %q", i, got[i], n)
		}
	}
}
