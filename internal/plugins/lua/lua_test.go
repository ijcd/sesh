package lua

import (
	"context"
	"os"
	"path/filepath"
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
		if n == "emacs-lua" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("emacs-lua not in registered names: %v", names)
	}
	p, _ := reg.Get("emacs-lua")
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
	argv := argvs[0]
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
	p, _ := reg.Get("emacs-lua")
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
	argv := argvs[0]
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
