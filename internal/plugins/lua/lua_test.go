package lua

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

// TestLua_SentinelPluginUpWritesFile drives a tiny inline Lua plugin
// whose up writes a file via sesh.exec; verifies the bridge end-to-end.
func TestLua_SentinelPluginUpWritesFile(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel.txt")

	src := `
sesh.register("sentinel", {
  up = function(env, cfg)
    local r = sesh.exec("touch", { cfg.path })
    if r.code ~= 0 then return "touch failed: " .. (r.stderr or "") end
    return nil
  end,
  down = function(env, cfg)
    sesh.exec("rm", { "-f", cfg.path })
    return nil
  end,
  validate = function(env, cfg)
    if not cfg.path then return { "path required" } end
    return {}
  end,
  dry_run = function(env, cfg)
    return { { "touch", cfg.path } }
  end,
})
`
	reg := plugins.NewRegistry()
	if err := loadFromString(src, reg.Register); err != nil {
		t.Fatalf("loadFromString: %v", err)
	}
	p, ok := reg.Get("sentinel")
	if !ok {
		t.Fatal("sentinel plugin not registered")
	}

	cfg := rawFromYAML(t, "path: "+sentinel+"\n")
	inst, err := p.New(plugins.ProjectEnv{Name: "spike", Cwd: dir}, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := inst.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel file missing: %v", err)
	}

	if errs := inst.Validate(); len(errs) != 0 {
		t.Errorf("Validate with path = empty errors, got %v", errs)
	}

	argvs, err := inst.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(argvs) != 1 {
		t.Fatalf("DryRun rows = %d, want 1: %v", len(argvs), argvs)
	}
	argv := argvs[0]
	if len(argv) != 2 || argv[0] != "touch" || argv[1] != sentinel {
		t.Errorf("DryRun = %v, want [touch %s]", argv, sentinel)
	}

	if err := inst.Down(context.Background()); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Errorf("expected sentinel removed, stat err = %v", err)
	}
}

// TestLua_ValidateMissingFieldReturnsError exercises validate's table-of-strings return.
func TestLua_ValidateMissingFieldReturnsError(t *testing.T) {
	src := `
sesh.register("v", {
  up = function() return nil end,
  down = function() return nil end,
  validate = function(env, cfg)
    local errs = {}
    if not cfg.path then table.insert(errs, "path required") end
    return errs
  end,
  dry_run = function() return {} end,
})
`
	reg := plugins.NewRegistry()
	if err := loadFromString(src, reg.Register); err != nil {
		t.Fatalf("loadFromString: %v", err)
	}
	p, _ := reg.Get("v")
	inst, err := p.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, plugins.NewRawConfig(nil))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	errs := inst.Validate()
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "path required") {
		t.Errorf("Validate = %v, want [path required]", errs)
	}
}

// TestLua_UpReturnsStringPropagatesError verifies up→string→Go error round-trip.
func TestLua_UpReturnsStringPropagatesError(t *testing.T) {
	src := `
sesh.register("oops", {
  up = function() return "kaboom" end,
  down = function() return nil end,
})
`
	reg := plugins.NewRegistry()
	if err := loadFromString(src, reg.Register); err != nil {
		t.Fatalf("loadFromString: %v", err)
	}
	p, _ := reg.Get("oops")
	inst, _ := p.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, plugins.NewRawConfig(nil))
	err := inst.Up(context.Background())
	if err == nil || !strings.Contains(err.Error(), "kaboom") {
		t.Errorf("Up err = %v, want containing kaboom", err)
	}
}

// TestLua_UpRuntimeErrorPropagates verifies Lua error() calls surface as Go errors.
func TestLua_UpRuntimeErrorPropagates(t *testing.T) {
	src := `
sesh.register("bang", {
  up = function() error("boom from lua") end,
  down = function() return nil end,
})
`
	reg := plugins.NewRegistry()
	if err := loadFromString(src, reg.Register); err != nil {
		t.Fatalf("loadFromString: %v", err)
	}
	p, _ := reg.Get("bang")
	inst, _ := p.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, plugins.NewRawConfig(nil))
	err := inst.Up(context.Background())
	if err == nil || !strings.Contains(err.Error(), "boom from lua") {
		t.Errorf("Up err = %v, want containing 'boom from lua'", err)
	}
}

// TestLua_EmbeddedEmacsDryRun exercises the ported emacs.lua via DryRun
// without requiring real emacs installed.
func TestLua_EmbeddedEmacsDryRun(t *testing.T) {
	reg := plugins.NewRegistry()
	names, err := LoadAll(reg.Register)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	found := false
	for _, n := range names {
		if n == "emacs" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("emacs not in registered names: %v", names)
	}
	p, _ := reg.Get("emacs")
	inst, err := p.New(
		plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"},
		rawFromYAML(t, ""),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	argvs, err := inst.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(argvs) == 0 {
		t.Fatalf("DryRun returned no rows")
	}
	// Open-dispatch is the last row in the 3-row probe/spawn/open sequence.
	argv := argvs[len(argvs)-1]
	if len(argv) != 4 {
		t.Fatalf("argv len = %d, want 4: %v", len(argv), argv)
	}
	if argv[0] != "emacsclient" {
		t.Errorf("argv[0] = %q, want emacsclient", argv[0])
	}
	if argv[1] != "--socket-name=sesh" {
		t.Errorf("argv[1] = %q, want --socket-name=sesh", argv[1])
	}
	if argv[2] != "-e" {
		t.Errorf("argv[2] = %q, want -e", argv[2])
	}
	want := `(sesh-open-project "lib" "/cwd")`
	if argv[3] != want {
		t.Errorf("argv[3] = %q, want %q", argv[3], want)
	}
}

// TestLua_EmbeddedEmacsDryRunWithConfigOverrides exercises YAML→Lua-table
// round-trip with mixed scalars and a list.
func TestLua_EmbeddedEmacsDryRunWithConfigOverrides(t *testing.T) {
	reg := plugins.NewRegistry()
	if _, err := LoadAll(reg.Register); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	p, _ := reg.Get("emacs")
	src := "hook: my-open\ndaemon: prj\nfiles:\n  - README.md\n  - /abs/path.txt\n"
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, src))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	argvs, err := inst.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(argvs) == 0 {
		t.Fatalf("DryRun returned no rows")
	}
	// Open-dispatch is the last row in the 3-row probe/spawn/open sequence.
	argv := argvs[len(argvs)-1]
	if argv[1] != "--socket-name=prj" {
		t.Errorf("argv[1] = %q, want --socket-name=prj", argv[1])
	}
	if !strings.Contains(argv[3], "(my-open ") {
		t.Errorf("argv[3] missing custom hook: %q", argv[3])
	}
	if !strings.Contains(argv[3], `"/cwd/README.md"`) {
		t.Errorf("argv[3] missing absolutized README path: %q", argv[3])
	}
	if !strings.Contains(argv[3], `"/abs/path.txt"`) {
		t.Errorf("argv[3] missing absolute path: %q", argv[3])
	}
}

// TestLua_DuplicateRegisterErrors checks sesh.register rejects duplicates.
func TestLua_DuplicateRegisterErrors(t *testing.T) {
	src := `
sesh.register("dup", { up = function() end, down = function() end })
sesh.register("dup", { up = function() end, down = function() end })
`
	reg := plugins.NewRegistry()
	err := loadFromString(src, reg.Register)
	if err == nil || !strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("expected duplicate-name error, got %v", err)
	}
}

// TestBridge_OneLStatePerPlugin verifies plugins do not share globals.
// Plugin A sets _G.shared_marker; plugin B reads it. With per-plugin
// LState, B never sees A's global and returns nil. With a shared state
// (spike behavior), B would see the marker and return an error.
func TestBridge_OneLStatePerPlugin(t *testing.T) {
	src := `
sesh.register("plugA", {
  up = function(env, cfg) _G.shared_marker = "from-A"; return nil end,
  down = function() return nil end,
})
sesh.register("plugB", {
  up = function(env, cfg)
    if _G.shared_marker then
      return "plugin B saw shared_marker = " .. _G.shared_marker
    end
    return nil
  end,
  down = function() return nil end,
})
`
	reg := plugins.NewRegistry()
	if err := loadFromString(src, reg.Register); err != nil {
		t.Fatalf("loadFromString: %v", err)
	}
	pa, _ := reg.Get("plugA")
	pb, _ := reg.Get("plugB")
	ia, err := pa.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, plugins.NewRawConfig(nil))
	if err != nil {
		t.Fatalf("plugA New: %v", err)
	}
	ib, err := pb.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, plugins.NewRawConfig(nil))
	if err != nil {
		t.Fatalf("plugB New: %v", err)
	}
	if err := ia.Up(context.Background()); err != nil {
		t.Fatalf("plugA Up: %v", err)
	}
	if err := ib.Up(context.Background()); err != nil {
		t.Errorf("plugB Up: expected nil (isolated state), got %v", err)
	}
}

// TestWaitFor_PredicateErrorPropagates verifies a predicate that raises
// surfaces as a Go error rather than silently timing out.
func TestWaitFor_PredicateErrorPropagates(t *testing.T) {
	src := `
sesh.register("wfbad", {
  up = function(env, cfg)
    sesh.wait_for(function() error("boom from predicate") end, 5000)
    return nil
  end,
  down = function() return nil end,
})
`
	reg := plugins.NewRegistry()
	if err := loadFromString(src, reg.Register); err != nil {
		t.Fatalf("loadFromString: %v", err)
	}
	p, _ := reg.Get("wfbad")
	inst, err := p.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, plugins.NewRawConfig(nil))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	start := time.Now()
	err = inst.Up(context.Background())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Up: expected error from predicate, got nil")
	}
	if !strings.Contains(err.Error(), "boom") && !strings.Contains(err.Error(), "predicate") {
		t.Errorf("Up err = %v, want containing 'boom' or 'predicate'", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Up took %v, expected early return (well under 5s timeout)", elapsed)
	}
}

// TestApplescript_DispatchesViaOsascript verifies sesh.applescript invokes
// osascript -e <script> via the test seam. Skips on non-darwin platforms.
func TestApplescript_DispatchesViaOsascript(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only: osascript not available on this platform")
	}
	var gotArgs []string
	orig := execOsascript
	execOsascript = func(args ...string) ([]byte, int, error) {
		gotArgs = append([]string{}, args...)
		return []byte("ok"), 0, nil
	}
	t.Cleanup(func() { execOsascript = orig })

	src := `
sesh.register("asrun", {
  up = function(env, cfg)
    local r = sesh.applescript("tell application \"Firefox\" to activate")
    if r.code ~= 0 then return "applescript failed: " .. (r.error or "") end
    if r.stdout ~= "ok" then return "stdout = " .. r.stdout end
    return nil
  end,
  down = function() return nil end,
})
`
	reg := plugins.NewRegistry()
	if err := loadFromString(src, reg.Register); err != nil {
		t.Fatalf("loadFromString: %v", err)
	}
	p, _ := reg.Get("asrun")
	inst, err := p.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, plugins.NewRawConfig(nil))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := inst.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "-e" {
		t.Errorf("argv = %v, want [-e <script>]", gotArgs)
	}
	if !strings.Contains(gotArgs[1], "Firefox") {
		t.Errorf("argv[1] = %q, want containing 'Firefox'", gotArgs[1])
	}
}

// --- Emacs Lua-port test ladder ---------------------------------------
//
// Mirrors the v0.4 Go emacs test coverage (internal/plugins/emacs/emacs_test.go)
// against the Lua-loaded plugin. All tests use the execRunner / pathLookup
// seams to script responses without spawning real emacs. Names track the
// Go tests with an EmacsLua_ prefix for grep parity.

// loadEmacsLuaPlugin loads the embedded emacs.lua and returns its plugin.
func loadEmacsLuaPlugin(t *testing.T) plugins.Plugin {
	t.Helper()
	reg := plugins.NewRegistry()
	if _, err := LoadAll(reg.Register); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	p, ok := reg.Get("emacs")
	if !ok {
		t.Fatal("emacs plugin not registered")
	}
	return p
}

// execScript is a recording, scripted execRunner. results are consumed
// in order; once exhausted, returns success (code 0). All calls land in
// calls as joined "bin arg arg" strings for assertion.
type execScript struct {
	calls   []string
	results []execResult
}

func (e *execScript) run(bin string, args []string) execResult {
	argv := append([]string{bin}, args...)
	e.calls = append(e.calls, strings.Join(argv, " "))
	if len(e.results) > 0 {
		r := e.results[0]
		e.results = e.results[1:]
		return r
	}
	return execResult{Code: 0}
}

// withExecScript swaps execRunner for the duration of the test.
func withExecScript(t *testing.T, s *execScript) {
	t.Helper()
	orig := execRunner
	execRunner = s.run
	t.Cleanup(func() { execRunner = orig })
}

// withPathLookup swaps pathLookup for the duration of the test.
func withPathLookup(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	orig := pathLookup
	pathLookup = fn
	t.Cleanup(func() { pathLookup = orig })
}

func TestEmacsLua_PluginNameIsEmacs(t *testing.T) {
	p := loadEmacsLuaPlugin(t)
	if got := p.Name(); got != "emacs" {
		t.Errorf("Name = %q, want %q", got, "emacs")
	}
}

func TestEmacsLua_NewAppliesConventionDefaults(t *testing.T) {
	p := loadEmacsLuaPlugin(t)
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	argvs, err := inst.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(argvs) != 3 {
		t.Fatalf("DryRun rows = %d, want 3: %v", len(argvs), argvs)
	}
	// probe + open both use --socket-name=sesh; spawn uses --daemon=sesh
	probe := strings.Join(argvs[0], " ")
	spawn := strings.Join(argvs[1], " ")
	open := strings.Join(argvs[2], " ")
	if !strings.Contains(probe, "--socket-name=sesh") {
		t.Errorf("probe missing default daemon: %q", probe)
	}
	if !strings.Contains(spawn, "--daemon=sesh") {
		t.Errorf("spawn missing default daemon: %q", spawn)
	}
	if !strings.Contains(open, `(sesh-open-project "lib" "/cwd")`) {
		t.Errorf("open form missing default hook: %q", open)
	}
}

func TestEmacsLua_NewRespectsConfigOverrides(t *testing.T) {
	p := loadEmacsLuaPlugin(t)
	src := "hook: my-open\nclose_hook: my-close\ndaemon: prj\nfiles:\n  - README.md\n  - /abs/path.txt\n"
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, src))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	argvs, err := inst.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(argvs) != 3 {
		t.Fatalf("DryRun rows = %d, want 3: %v", len(argvs), argvs)
	}
	probe := strings.Join(argvs[0], " ")
	spawn := strings.Join(argvs[1], " ")
	open := strings.Join(argvs[2], " ")
	if !strings.Contains(probe, "--socket-name=prj") {
		t.Errorf("probe missing override daemon: %q", probe)
	}
	if !strings.Contains(spawn, "--daemon=prj") {
		t.Errorf("spawn missing override daemon: %q", spawn)
	}
	if !strings.Contains(open, "(my-open ") {
		t.Errorf("open missing custom hook: %q", open)
	}
	if !strings.Contains(open, `"/cwd/README.md"`) {
		t.Errorf("open missing absolutized README: %q", open)
	}
	if !strings.Contains(open, `"/abs/path.txt"`) {
		t.Errorf("open missing absolute path: %q", open)
	}
}

func TestEmacsLua_DryRunIncludesProbeSpawnAndOpenArgv(t *testing.T) {
	p := loadEmacsLuaPlugin(t)
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	argvs, err := inst.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(argvs) != 3 {
		t.Fatalf("rows = %d, want 3: %v", len(argvs), argvs)
	}
	// 0: probe — emacsclient --socket-name=... -e (emacs-version)
	probe := argvs[0]
	if probe[0] != "emacsclient" {
		t.Errorf("probe[0] = %q, want emacsclient", probe[0])
	}
	if !strings.Contains(strings.Join(probe, " "), "--socket-name=") {
		t.Errorf("probe missing --socket-name=: %v", probe)
	}
	if probe[len(probe)-1] != "(emacs-version)" {
		t.Errorf("probe form = %q, want (emacs-version)", probe[len(probe)-1])
	}
	// 1: spawn — emacs --daemon=...
	spawn := argvs[1]
	if spawn[0] != "emacs" {
		t.Errorf("spawn[0] = %q, want emacs", spawn[0])
	}
	if !strings.HasPrefix(spawn[1], "--daemon=") {
		t.Errorf("spawn[1] = %q, want --daemon=...", spawn[1])
	}
	// 2: open dispatch — emacsclient ... (sesh-open-project ...)
	open := argvs[2]
	if open[0] != "emacsclient" {
		t.Errorf("open[0] = %q, want emacsclient", open[0])
	}
	if !strings.Contains(strings.Join(open, " "), "(sesh-open-project ") {
		t.Errorf("open missing hook call: %v", open)
	}
}

func TestEmacsLua_UpInvokesEmacsclientWithOpenFormHealthyDaemon(t *testing.T) {
	s := &execScript{
		// 0: probe succeeds → no daemon spawn
		// 1: open dispatch succeeds
		results: []execResult{
			{Code: 0}, // probe
			{Code: 0}, // open
		},
	}
	withExecScript(t, s)

	p := loadEmacsLuaPlugin(t)
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := inst.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(s.calls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d: %v", len(s.calls), s.calls)
	}
	if !strings.Contains(s.calls[0], "emacsclient") || !strings.Contains(s.calls[0], "(emacs-version)") {
		t.Errorf("call[0] not probe: %q", s.calls[0])
	}
	if !strings.Contains(s.calls[1], "emacsclient") || !strings.Contains(s.calls[1], "(sesh-open-project \"lib\" \"/cwd\")") {
		t.Errorf("call[1] not open dispatch: %q", s.calls[1])
	}
	// Defensive: no daemon spawn on healthy-daemon path.
	for _, c := range s.calls {
		if strings.Contains(c, "--daemon=") {
			t.Errorf("unexpected daemon spawn: %q", c)
		}
	}
}

func TestEmacsLua_UpSpawnsDaemonOnProbeFail(t *testing.T) {
	s := &execScript{
		// 0: probe fails → spawn daemon
		// 1: emacs --daemon=sesh succeeds
		// 2: post-spawn probe (via wait_for) succeeds
		// 3: open dispatch succeeds
		results: []execResult{
			{Code: 1, Stderr: "can't find socket"},
			{Code: 0},
			{Code: 0},
			{Code: 0},
		},
	}
	withExecScript(t, s)

	p := loadEmacsLuaPlugin(t)
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := inst.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(s.calls) < 3 {
		t.Fatalf("expected >=3 exec calls, got %d: %v", len(s.calls), s.calls)
	}
	foundSpawn, foundOpen := false, false
	for _, c := range s.calls {
		if strings.Contains(c, "emacs --daemon=sesh") {
			foundSpawn = true
		}
		if strings.Contains(c, "(sesh-open-project \"lib\" \"/cwd\")") {
			foundOpen = true
		}
	}
	if !foundSpawn {
		t.Errorf("expected emacs --daemon=sesh spawn in calls: %v", s.calls)
	}
	if !foundOpen {
		t.Errorf("expected open dispatch in calls: %v", s.calls)
	}
}

func TestEmacsLua_DownIdempotentNoSession(t *testing.T) {
	s := &execScript{
		// Close dispatch fails — Down must still return nil (best-effort).
		results: []execResult{
			{Code: 1, Stderr: "nope"},
		},
	}
	withExecScript(t, s)

	p := loadEmacsLuaPlugin(t)
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := inst.Down(context.Background()); err != nil {
		t.Errorf("Down returned error (best-effort contract): %v", err)
	}
	if len(s.calls) != 1 {
		t.Errorf("expected 1 close call, got %d: %v", len(s.calls), s.calls)
	}
}

func TestEmacsLua_ValidateMissingEmacsclient(t *testing.T) {
	withPathLookup(t, func(bin string) (string, error) {
		return "", errors.New("not found")
	})

	p := loadEmacsLuaPlugin(t)
	inst, err := p.New(plugins.ProjectEnv{Name: "lib", Cwd: "/cwd"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	errs := inst.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation error")
	}
	hit := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "emacsclient") {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("expected error mentioning emacsclient, got %v", errs)
	}
}

// --- Firefox Lua plugin test ladder -----------------------------------
//
// Mirrors the v0.5 Task 4 spec: firefox plugin shells out to
// `firefox -CreateProfile <name>` (idempotent) and `firefox -P <name>
// --new-instance [url]` (detached). Validate checks PATH or app-bundle
// fallback. Down is a no-op unless kill_on_down dispatches AppleScript.

// loadFirefoxLuaPlugin loads the embedded firefox.lua and returns its plugin.
func loadFirefoxLuaPlugin(t *testing.T) plugins.Plugin {
	t.Helper()
	reg := plugins.NewRegistry()
	if _, err := LoadAll(reg.Register); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	p, ok := reg.Get("firefox")
	if !ok {
		t.Fatal("firefox plugin not registered")
	}
	return p
}

// withExecDetachRecorder swaps execDetachRunner to capture argv.
func withExecDetachRecorder(t *testing.T, calls *[]string) {
	t.Helper()
	orig := execDetachRunner
	execDetachRunner = func(bin string, args []string) (int, error) {
		argv := append([]string{bin}, args...)
		*calls = append(*calls, strings.Join(argv, " "))
		return 12345, nil
	}
	t.Cleanup(func() { execDetachRunner = orig })
}

// withFileExists swaps fileExists for the duration of the test.
func withFileExists(t *testing.T, fn func(string) bool) {
	t.Helper()
	orig := fileExists
	fileExists = fn
	t.Cleanup(func() { fileExists = orig })
}

func TestFirefoxLua_UpRunsCreateThenLaunch(t *testing.T) {
	// Pretend firefox is on PATH so resolve_binary takes the PATH branch.
	withPathLookup(t, func(bin string) (string, error) {
		if bin == "firefox" {
			return "/usr/bin/firefox", nil
		}
		return "", errors.New("not found")
	})

	// Script the synchronous -CreateProfile call to return success.
	s := &execScript{results: []execResult{{Code: 0}}}
	withExecScript(t, s)

	var detachCalls []string
	withExecDetachRecorder(t, &detachCalls)

	p := loadFirefoxLuaPlugin(t)
	inst, err := p.New(plugins.ProjectEnv{Name: "liberties", Cwd: "/cwd"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := inst.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}

	if len(s.calls) != 1 {
		t.Fatalf("exec calls = %d, want 1: %v", len(s.calls), s.calls)
	}
	if !strings.Contains(s.calls[0], "-CreateProfile") || !strings.Contains(s.calls[0], "liberties") {
		t.Errorf("create call = %q, want -CreateProfile liberties", s.calls[0])
	}
	if len(detachCalls) != 1 {
		t.Fatalf("detach calls = %d, want 1: %v", len(detachCalls), detachCalls)
	}
	launch := detachCalls[0]
	if !strings.Contains(launch, "-P liberties") {
		t.Errorf("launch missing -P liberties: %q", launch)
	}
	if !strings.Contains(launch, "--new-instance") {
		t.Errorf("launch missing --new-instance: %q", launch)
	}
}

func TestFirefoxLua_KillOnDownInvokesApplescript(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only: osascript not available on this platform")
	}
	var gotArgs []string
	orig := execOsascript
	execOsascript = func(args ...string) ([]byte, int, error) {
		gotArgs = append([]string{}, args...)
		return []byte(""), 0, nil
	}
	t.Cleanup(func() { execOsascript = orig })

	p := loadFirefoxLuaPlugin(t)
	inst, err := p.New(
		plugins.ProjectEnv{Name: "liberties", Cwd: "/cwd"},
		rawFromYAML(t, "kill_on_down: true\n"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := inst.Down(context.Background()); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "-e" {
		t.Fatalf("osascript argv = %v, want [-e <script>]", gotArgs)
	}
	if !strings.Contains(gotArgs[1], "liberties") {
		t.Errorf("script missing profile name: %q", gotArgs[1])
	}
	if !strings.Contains(gotArgs[1], "Firefox") {
		t.Errorf("script missing Firefox: %q", gotArgs[1])
	}
}

func TestFirefoxLua_ValidateMissingBinary(t *testing.T) {
	withPathLookup(t, func(string) (string, error) { return "", errors.New("not found") })
	withFileExists(t, func(string) bool { return false })

	p := loadFirefoxLuaPlugin(t)
	inst, err := p.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, rawFromYAML(t, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	errs := inst.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation error")
	}
	hit := false
	for _, e := range errs {
		msg := e.Error()
		if strings.Contains(msg, "firefox") && strings.Contains(msg, "binary") {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("expected error mentioning firefox and binary, got %v", errs)
	}
}

func TestFirefoxLua_DryRunReturnsBothArgvs(t *testing.T) {
	withPathLookup(t, func(bin string) (string, error) {
		if bin == "firefox" {
			return "/usr/bin/firefox", nil
		}
		return "", errors.New("not found")
	})

	p := loadFirefoxLuaPlugin(t)
	inst, err := p.New(
		plugins.ProjectEnv{Name: "liberties", Cwd: "/cwd"},
		rawFromYAML(t, "url: https://example.com\n"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	argvs, err := inst.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(argvs) != 2 {
		t.Fatalf("rows = %d, want 2: %v", len(argvs), argvs)
	}
	create := strings.Join(argvs[0], " ")
	launch := strings.Join(argvs[1], " ")
	if !strings.Contains(create, "-CreateProfile") || !strings.Contains(create, "liberties") {
		t.Errorf("create row = %q, want -CreateProfile + liberties", create)
	}
	if !strings.Contains(launch, "-P liberties") {
		t.Errorf("launch row missing -P liberties: %q", launch)
	}
	if !strings.Contains(launch, "--new-instance") {
		t.Errorf("launch row missing --new-instance: %q", launch)
	}
	if !strings.Contains(launch, "https://example.com") {
		t.Errorf("launch row missing url: %q", launch)
	}
}

// TestLua_PathLookupHandlesMissing verifies sesh.path_lookup returns nil on miss.
func TestLua_PathLookupHandlesMissing(t *testing.T) {
	src := `
sesh.register("p", {
  up = function()
    if sesh.path_lookup("definitely-not-installed-xyz123") then
      return "should have been nil"
    end
    return nil
  end,
  down = function() return nil end,
})
`
	reg := plugins.NewRegistry()
	if err := loadFromString(src, reg.Register); err != nil {
		t.Fatalf("loadFromString: %v", err)
	}
	p, _ := reg.Get("p")
	inst, _ := p.New(plugins.ProjectEnv{Name: "x", Cwd: "/"}, plugins.NewRawConfig(nil))
	if err := inst.Up(context.Background()); err != nil {
		t.Errorf("Up: %v", err)
	}
}
