# sesh v0.2 — kitty driver — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `kitty` driver to sesh, supporting all three kitty containment pairs (leaf, tmux, kitty), a `--launch` CLI flag for non-kitty terminals, and exhaustive test coverage (target: ~80 new tests).

**Architecture:** Mirrors the v0.1 tmux driver shape — `Runner` seam, per-command remote control via `kitten @`, BuildCommands returns shell-string commands that the driver executes one-by-one. Cross-driver dispatch (kitty hosting tmux) is engine-level, made possible by a small `Driver.AttachCommand` interface extension. `--launch` lives in the cmd layer: it spawns kitty, sets `KITTY_LISTEN_ON` env, writes a state file for later teardown, then proceeds to the standard engine.Up flow.

**Tech Stack:** Go 1.22+, stdlib only (no new external deps). Tests follow existing v0.1 patterns: table-driven units, fakeRunner seam for driver tests, separate build tags `integration_kitty` for real-kitty tests, `e2e` for binary-level tests.

**Layout:**

```
internal/drivers/driver.go             EXTEND: add AttachCommand method to interface
internal/drivers/tmux/driver.go        EXTEND: implement AttachCommand
internal/drivers/mock/mock.go          EXTEND: implement AttachCommand
internal/state/                        NEW package: state.json read/write with flock
internal/drivers/kitty/                NEW package
  runner.go                              Runner interface, execRunner, kittenPath()
  layouts.go                             ValidLayouts set + ValidateLayout()
  match.go                               BuildMatch helpers (regex escape)
  session.go                             BuildCommands (leaf, multi-pane, multi-tab, focus-tab)
  driver.go                              Driver impl (Name/Up/Down/Status/Validate/DryRun/AttachCommand)
  capture.go                             Parse `kitten ls` JSON
internal/drivers/kitty/launch/         NEW package
  launch.go                              spawn kitty + wait for socket
internal/engine/up.go                  EXTEND: cross-driver tab dispatch
internal/engine/engine.go              EXTEND: Validate calls driver.Validate
cmd/sesh/root.go                       EXTEND: register kitty.New()
cmd/sesh/up.go                         EXTEND: --launch flag
cmd/sesh/local.go                      EXTEND: --launch flag
cmd/sesh/down.go                       EXTEND: honor state.json for --launch cleanup
docs/superpowers/specs/2026-05-09-sesh-kitty-driver-design.md   reference (already exists)
```

Tests are co-located with code (`_test.go` next to source). Integration tests use build tags.

---

## Task 1: Driver.AttachCommand interface extension

**Files:**
- Modify: `internal/drivers/driver.go`
- Modify: `internal/drivers/tmux/driver.go`
- Modify: `internal/drivers/tmux/driver_test.go`
- Modify: `internal/drivers/mock/mock.go`

- [ ] **Step 1: Write the failing tests for tmux AttachCommand**

Append to `internal/drivers/tmux/driver_test.go`:
```go
func TestDriver_AttachCommand_DefaultSlug(t *testing.T) {
    d := New()
    p := &spec.Project{Name: "Liberties Demo", Driver: "tmux"}
    cmd, err := d.AttachCommand(p)
    if err != nil {
        t.Fatal(err)
    }
    if cmd != "tmux attach -t liberties-demo" {
        t.Errorf("AttachCommand = %q, want %q", cmd, "tmux attach -t liberties-demo")
    }
}

func TestDriver_AttachCommand_ExplicitSession(t *testing.T) {
    d := New()
    p := &spec.Project{Name: "x", Driver: "tmux", Session: "custom-sess"}
    cmd, err := d.AttachCommand(p)
    if err != nil {
        t.Fatal(err)
    }
    if cmd != "tmux attach -t custom-sess" {
        t.Errorf("AttachCommand = %q", cmd)
    }
}
```

- [ ] **Step 2: Run tests, confirm failure**

```bash
go test ./internal/drivers/tmux/...
```
Expected: build failure — `AttachCommand` undefined on `*Driver`.

- [ ] **Step 3: Extend the Driver interface**

Edit `internal/drivers/driver.go`. Add to the interface:
```go
// AttachCommand returns the shell command a parent driver should run to
// attach to (host) this driver's container. Returns error if the driver
// has no externally-attachable concept (e.g., kitty has no detached
// sessions). Used by engine for cross-driver tab dispatch.
AttachCommand(p *spec.Project) (string, error)
```

- [ ] **Step 4: Implement on tmux Driver**

Add to `internal/drivers/tmux/driver.go`:
```go
// AttachCommand returns "tmux attach -t <session>" for the project.
func (d *Driver) AttachCommand(p *spec.Project) (string, error) {
    sess := p.Session
    if sess == "" {
        sess = Slug(p.Name)
    }
    return fmt.Sprintf("tmux attach -t %s", sess), nil
}
```

(`Slug` was exported in T34 of the v0.1 plan — already public.)

- [ ] **Step 5: Implement on mock.Driver**

Add to `internal/drivers/mock/mock.go`:
```go
// AttachCommandVal is what AttachCommand returns. AttachCommandErr is the error.
// Set these in tests as needed; defaults are empty (returns "" and nil).

func (d *Driver) AttachCommand(p *spec.Project) (string, error) {
    return d.AttachCommandVal, d.AttachCommandErr
}
```

And add fields to the `Driver` struct:
```go
AttachCommandVal string
AttachCommandErr error
```

- [ ] **Step 6: Run tests, confirm pass**

```bash
go test ./...
```
Expected: PASS for all 81+ tests.

- [ ] **Step 7: Commit**

```bash
git add internal/drivers/
git commit -m "drivers: add AttachCommand to Driver interface (tmux + mock)"
```

---

## Task 2: internal/state package

**Files:**
- Create: `internal/state/state.go`
- Create: `internal/state/state_test.go`

- [ ] **Step 1: Write failing tests**

`internal/state/state_test.go`:
```go
package state

import (
    "os"
    "path/filepath"
    "sync"
    "testing"
    "time"
)

func TestStore_LoadEmpty(t *testing.T) {
    dir := t.TempDir()
    s, err := Load(filepath.Join(dir, "state.json"))
    if err != nil {
        t.Fatal(err)
    }
    if len(s.Projects) != 0 {
        t.Errorf("expected empty Projects, got %v", s.Projects)
    }
}

func TestStore_LoadCorruptedReturnsError(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "state.json")
    if err := os.WriteFile(p, []byte("not json"), 0o644); err != nil {
        t.Fatal(err)
    }
    _, err := Load(p)
    if err == nil {
        t.Fatal("expected error on corrupted JSON")
    }
}

func TestStore_SetAndPersist(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "state.json")
    s, _ := Load(p)
    s.Set("liberties", LaunchEntry{
        Socket: "/tmp/sock", Pid: 1234, LaunchedAt: time.Now(),
    })
    if err := s.Save(p); err != nil {
        t.Fatal(err)
    }

    // Reload from disk
    s2, err := Load(p)
    if err != nil {
        t.Fatal(err)
    }
    e, ok := s2.Get("liberties")
    if !ok {
        t.Fatal("entry not found after reload")
    }
    if e.Socket != "/tmp/sock" || e.Pid != 1234 {
        t.Errorf("got entry %+v", e)
    }
}

func TestStore_Delete(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "state.json")
    s, _ := Load(p)
    s.Set("a", LaunchEntry{Socket: "/a", Pid: 1})
    s.Set("b", LaunchEntry{Socket: "/b", Pid: 2})
    s.Delete("a")
    if _, ok := s.Get("a"); ok {
        t.Error("a should be gone")
    }
    if _, ok := s.Get("b"); !ok {
        t.Error("b should still exist")
    }
}

func TestStore_AtomicWriteSurvivesPartialFailure(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "state.json")
    s, _ := Load(p)
    s.Set("x", LaunchEntry{Socket: "/x", Pid: 1})
    if err := s.Save(p); err != nil {
        t.Fatal(err)
    }
    // Verify there's no leftover .tmp file
    if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
        t.Errorf("temp file should be gone, stat: %v", err)
    }
}

func TestStore_ConcurrentSavesSerialized(t *testing.T) {
    // Smoke test: two goroutines saving concurrently shouldn't corrupt.
    dir := t.TempDir()
    p := filepath.Join(dir, "state.json")

    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            s, _ := Load(p)
            s.Set("p", LaunchEntry{Socket: "/x", Pid: n})
            _ = s.Save(p)
        }(i)
    }
    wg.Wait()

    // Final state should be valid JSON.
    s, err := Load(p)
    if err != nil {
        t.Fatalf("post-concurrency load failed: %v", err)
    }
    if _, ok := s.Get("p"); !ok {
        t.Error("entry lost")
    }
}
```

- [ ] **Step 2: Run tests, confirm failure**

```bash
go test ./internal/state/...
```
Expected: build failure — package undefined.

- [ ] **Step 3: Implement state.go**

`internal/state/state.go`:
```go
// Package state persists per-project runtime state to a JSON file.
// Currently used by the kitty driver's --launch flow to track which
// projects have a sesh-spawned kitty so we can clean up on `sesh down`.
package state

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "syscall"
    "time"
)

const Version = 1

type LaunchEntry struct {
    Socket     string    `json:"socket"`
    Pid        int       `json:"pid"`
    LaunchedAt time.Time `json:"launched_at"`
}

type Store struct {
    Version  int                    `json:"version"`
    Projects map[string]LaunchEntry `json:"projects"`

    mu sync.Mutex
}

// Load reads state from path. Missing file → empty Store. Corrupted → error.
func Load(path string) (*Store, error) {
    s := &Store{Version: Version, Projects: map[string]LaunchEntry{}}
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return s, nil
        }
        return nil, fmt.Errorf("read %s: %w", path, err)
    }
    if len(data) == 0 {
        return s, nil
    }
    if err := json.Unmarshal(data, s); err != nil {
        return nil, fmt.Errorf("parse %s: %w", path, err)
    }
    if s.Projects == nil {
        s.Projects = map[string]LaunchEntry{}
    }
    return s, nil
}

func (s *Store) Get(name string) (LaunchEntry, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    e, ok := s.Projects[name]
    return e, ok
}

func (s *Store) Set(name string, e LaunchEntry) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.Projects[name] = e
}

func (s *Store) Delete(name string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.Projects, name)
}

// Save writes the state to path atomically (via tmp + rename) under a
// flock held for the duration of write.
func (s *Store) Save(path string) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    lockPath := path + ".lock"
    lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
    if err != nil {
        return fmt.Errorf("open lock %s: %w", lockPath, err)
    }
    defer lf.Close()
    if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
        return fmt.Errorf("flock %s: %w", lockPath, err)
    }
    defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)

    s.mu.Lock()
    s.Version = Version
    data, err := json.MarshalIndent(s, "", "  ")
    s.mu.Unlock()
    if err != nil {
        return fmt.Errorf("marshal: %w", err)
    }

    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return fmt.Errorf("write %s: %w", tmp, err)
    }
    if err := os.Rename(tmp, path); err != nil {
        return fmt.Errorf("rename %s → %s: %w", tmp, path, err)
    }
    return nil
}

// DefaultPath returns the standard state.json location for sesh.
func DefaultPath() (string, error) {
    if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
        return filepath.Join(xdg, "sesh", "state.json"), nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".local", "state", "sesh", "state.json"), nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/state/...
```
Expected: PASS for 6 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/state/
git commit -m "state: persist project → launch-socket mapping (atomic+flock)"
```

---

## Task 3: engine.Validate calls driver.Validate

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/up.go`
- Create: `internal/engine/validate.go`
- Create: `internal/engine/validate_test.go`

- [ ] **Step 1: Write failing test**

`internal/engine/validate_test.go`:
```go
package engine

import (
    "context"
    "errors"
    "strings"
    "testing"

    "github.com/ijcd/sesh/internal/drivers"
    "github.com/ijcd/sesh/internal/drivers/mock"
    "github.com/ijcd/sesh/internal/spec"
)

type validatingMock struct {
    *mock.Driver
    validateErrs []error
}

func (v *validatingMock) Validate(p *spec.Project) []error { return v.validateErrs }

func TestValidate_DriverErrorsBubble(t *testing.T) {
    md := mock.New("tmux")
    vm := &validatingMock{Driver: md, validateErrs: []error{errors.New("bad layout: tall is not a tmux layout")}}
    e := New()
    e.Register(vm)
    p := &spec.Project{Name: "x", Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}}}
    err := e.Validate(context.Background(), p)
    if err == nil {
        t.Fatal("expected error")
    }
    if !strings.Contains(err.Error(), "bad layout") {
        t.Errorf("got %v", err)
    }
}

func TestValidate_NoErrorsWhenDriverHappy(t *testing.T) {
    md := mock.New("tmux")
    e := New()
    e.Register(md)
    p := &spec.Project{Name: "x", Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}}}
    if err := e.Validate(context.Background(), p); err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}

func TestValidate_ContainmentErrorBubbles(t *testing.T) {
    md := mock.New("tmux")
    e := New()
    e.Register(md)
    p := &spec.Project{Name: "x", Driver: "tmux", Tabs: []spec.Tab{{
        Title: "x", Driver: "kitty", Panes: []spec.Pane{{Title: "p", Cmd: "y"}},
    }}}
    if err := e.Validate(context.Background(), p); err == nil {
        t.Fatal("expected containment error")
    }
}

// Make sure mock satisfies drivers.Driver after adding Validate.
var _ drivers.Driver = (*validatingMock)(nil)
```

But wait — `mock.Driver` doesn't yet have a `Validate` method. To make `validatingMock` work as drivers.Driver, the embedded mock needs a default no-op Validate. Add it in step 2 below.

- [ ] **Step 2: Add Validate to mock.Driver (returns nil errors by default)**

Edit `internal/drivers/mock/mock.go`. Add field + method:
```go
ValidateErrs []error

func (d *Driver) Validate(p *spec.Project) []error { return d.ValidateErrs }
```

This requires extending the `drivers.Driver` interface to include `Validate` — do that next.

- [ ] **Step 3: Add Validate to drivers.Driver interface**

Edit `internal/drivers/driver.go`. Add to the interface:
```go
// Validate runs driver-specific structural checks (layouts, etc.) and
// returns any violations. Engine-level validation (containment, schema)
// is separate; this is for driver internals.
Validate(p *spec.Project) []error
```

- [ ] **Step 4: Add no-op Validate to tmux Driver**

Edit `internal/drivers/tmux/driver.go`:
```go
// Validate runs tmux-specific checks. Currently a no-op; layouts are
// passed through to tmux without sesh-side validation.
func (d *Driver) Validate(p *spec.Project) []error { return nil }
```

- [ ] **Step 5: Run tests on existing packages, fix compile breaks**

```bash
go build ./...
```

If anything is broken: any `drivers.Driver`-typed variable now needs the new methods. Should already be addressed by the mock + tmux additions above.

- [ ] **Step 6: Implement engine.Validate**

Create `internal/engine/validate.go`:
```go
package engine

import (
    "context"
    "errors"
    "fmt"

    "github.com/ijcd/sesh/internal/spec"
)

// Validate runs engine-level checks (containment) and driver-level checks.
// Returns nil if all pass; otherwise an aggregated error.
func (e *Engine) Validate(_ context.Context, p *spec.Project) error {
    var errs []error
    if err := CheckContainment(p); err != nil {
        errs = append(errs, err)
    }
    d, err := e.driverFor(p.Driver)
    if err != nil {
        errs = append(errs, err)
    } else {
        for _, ve := range d.Validate(p) {
            errs = append(errs, ve)
        }
    }
    // Tab-driver overrides also get their driver's Validate.
    seen := map[string]bool{p.Driver: true}
    for _, t := range p.Tabs {
        if t.Driver == "" || seen[t.Driver] {
            continue
        }
        seen[t.Driver] = true
        td, err := e.driverFor(t.Driver)
        if err != nil {
            errs = append(errs, err)
            continue
        }
        for _, ve := range td.Validate(p) {
            errs = append(errs, ve)
        }
    }
    if len(errs) == 0 {
        return nil
    }
    return fmt.Errorf("validation failed: %w", errors.Join(errs...))
}
```

- [ ] **Step 7: Run all tests**

```bash
go test ./...
```
Expected: PASS — all existing tests still green; 3 new ones for validate.

- [ ] **Step 8: Update cmd/sesh/validate.go to use engine.Validate**

Edit `cmd/sesh/validate.go` — after `config.Load(...)`, replace the existing `engine.CheckContainment` call (added in v0.1 fix) with `e.Validate(ctx, p)`:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    p, err := config.Load(args[0], e.Drivers(), nil)
    if err != nil {
        return err
    }
    if err := e.Validate(context.Background(), p); err != nil {
        return err
    }
    fmt.Printf("ok: %s (%d tab(s), driver=%s)\n", p.Name, len(p.Tabs), p.Driver)
    return nil
},
```

- [ ] **Step 9: Commit**

```bash
git add internal/engine/ internal/drivers/ cmd/sesh/validate.go
git commit -m "engine: Validate calls driver.Validate (per-driver checks)"
```

---

## Task 4: kitty Runner + kittenPath

**Files:**
- Create: `internal/drivers/kitty/runner.go`
- Create: `internal/drivers/kitty/runner_test.go`

- [ ] **Step 1: Write failing tests**

`internal/drivers/kitty/runner_test.go`:
```go
package kitty

import (
    "context"
    "errors"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestKittenPath_PrefersPATH(t *testing.T) {
    dir := t.TempDir()
    fake := filepath.Join(dir, "kitten")
    if err := os.WriteFile(fake, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
        t.Fatal(err)
    }
    t.Setenv("PATH", dir)
    p, err := kittenPath()
    if err != nil {
        t.Fatal(err)
    }
    if p != fake {
        t.Errorf("got %q, want %q", p, fake)
    }
}

func TestKittenPath_ErrorWhenMissing(t *testing.T) {
    t.Setenv("PATH", t.TempDir())
    _, err := kittenPath()
    if err == nil {
        t.Fatal("expected error")
    }
    if !strings.Contains(err.Error(), "kitten not found") {
        t.Errorf("got %v", err)
    }
}

type fakeRunner struct {
    runs    []string
    runErr  error
    captureOut string
    captureErr error
}

func (f *fakeRunner) Run(ctx context.Context, args ...string) error {
    f.runs = append(f.runs, "kitten "+strings.Join(args, " "))
    return f.runErr
}
func (f *fakeRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
    return f.captureOut, f.captureErr
}

func TestExecRunner_PrefixesSocketArgs(t *testing.T) {
    // Verify that NewExecRunner injects --to <socket> before user args.
    // We can't actually exec without kitten; just check the wrapper logic.
    r := &execRunner{kittenPath: "/bin/echo", socket: "unix:/tmp/sock"}
    args := r.fullArgs([]string{"launch", "--type=tab"})
    want := []string{"@", "--to", "unix:/tmp/sock", "launch", "--type=tab"}
    if len(args) != len(want) {
        t.Fatalf("len mismatch: got %v, want %v", args, want)
    }
    for i := range want {
        if args[i] != want[i] {
            t.Errorf("arg[%d] = %q, want %q", i, args[i], want[i])
        }
    }
}

func TestExecRunner_ErrorsWhenSocketEmpty(t *testing.T) {
    r := &execRunner{kittenPath: "/bin/echo", socket: ""}
    err := r.Run(context.Background(), "ls")
    if err == nil {
        t.Fatal("expected error for empty socket")
    }
    if !strings.Contains(err.Error(), "KITTY_LISTEN_ON") {
        t.Errorf("got %v", err)
    }
    // To silence unused variable warning if errors stays imported:
    _ = errors.New
}
```

- [ ] **Step 2: Run tests, confirm failure**

```bash
go test ./internal/drivers/kitty/...
```
Expected: build failure — package undefined.

- [ ] **Step 3: Implement runner.go**

`internal/drivers/kitty/runner.go`:
```go
// Package kitty is the kitty driver.
package kitty

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "os"
    "os/exec"
)

// Runner is the seam for shelling out to kitten. Production uses execRunner
// (forks `kitten @ --to <socket> ...`); tests substitute a fake.
type Runner interface {
    Run(ctx context.Context, args ...string) error
    RunCapture(ctx context.Context, args ...string) (string, error)
}

type execRunner struct {
    kittenPath string
    socket     string // "" until KITTY_LISTEN_ON is read at Up time
}

func NewExecRunner() (*execRunner, error) {
    p, err := kittenPath()
    if err != nil {
        return nil, err
    }
    return &execRunner{kittenPath: p}, nil
}

// SetSocket assigns the kitten remote-control socket. Called by Driver
// methods just before issuing commands so KITTY_LISTEN_ON changes
// (e.g., from --launch) are picked up lazily.
func (r *execRunner) SetSocket(s string) { r.socket = s }

func (r *execRunner) fullArgs(args []string) []string {
    out := []string{"@"}
    if r.socket != "" {
        out = append(out, "--to", r.socket)
    }
    return append(out, args...)
}

func (r *execRunner) Run(ctx context.Context, args ...string) error {
    if r.socket == "" {
        return errors.New("kitty driver: KITTY_LISTEN_ON unset (run inside kitty or use --launch)")
    }
    cmd := exec.CommandContext(ctx, r.kittenPath, r.fullArgs(args)...)
    cmd.Stderr = os.Stderr
    return cmd.Run()
}

func (r *execRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
    if r.socket == "" {
        return "", errors.New("kitty driver: KITTY_LISTEN_ON unset (run inside kitty or use --launch)")
    }
    cmd := exec.CommandContext(ctx, r.kittenPath, r.fullArgs(args)...)
    var out bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = os.Stderr
    err := cmd.Run()
    return out.String(), err
}

// kittenPath finds the kitten binary. Order: PATH, kitty +kitten,
// macOS app-bundle paths.
func kittenPath() (string, error) {
    if p, err := exec.LookPath("kitten"); err == nil {
        return p, nil
    }
    for _, p := range []string{
        "/Applications/kitty.app/Contents/MacOS/kitten",
        "/opt/homebrew/bin/kitten",
        "/usr/local/bin/kitten",
    } {
        if _, err := os.Stat(p); err == nil {
            return p, nil
        }
    }
    return "", fmt.Errorf("kitten not found in PATH or known locations")
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS for 4 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/
git commit -m "kitty: Runner + kittenPath (PATH + macOS fallback)"
```

---

## Task 5: kitty layouts validator

**Files:**
- Create: `internal/drivers/kitty/layouts.go`
- Create: `internal/drivers/kitty/layouts_test.go`

- [ ] **Step 1: Write failing tests**

`internal/drivers/kitty/layouts_test.go`:
```go
package kitty

import (
    "strings"
    "testing"
)

func TestValidLayouts(t *testing.T) {
    for _, name := range []string{"splits", "tall", "fat", "grid", "horizontal", "vertical", "stack"} {
        if !IsValidLayout(name) {
            t.Errorf("%q should be valid", name)
        }
    }
}

func TestInvalidLayouts(t *testing.T) {
    for _, name := range []string{"main-vertical", "tiled", "even-horizontal", "garbage", "splits "} {
        if IsValidLayout(name) {
            t.Errorf("%q should be invalid", name)
        }
    }
}

func TestEmptyLayoutIsValid(t *testing.T) {
    // Empty means "use default"; not an error.
    if !IsValidLayout("") {
        t.Error("empty layout should be valid (default)")
    }
}

func TestDefaultLayout(t *testing.T) {
    if DefaultLayout != "splits" {
        t.Errorf("DefaultLayout = %q, want splits", DefaultLayout)
    }
}

func TestValidateLayoutErrorMessage(t *testing.T) {
    err := ValidateLayout("main-vertical")
    if err == nil {
        t.Fatal("expected error")
    }
    if !strings.Contains(err.Error(), "main-vertical") {
        t.Errorf("error should name the bad layout: %v", err)
    }
    if !strings.Contains(err.Error(), "splits") {
        t.Errorf("error should suggest valid layouts: %v", err)
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/... -run Layout
```
Expected: build failure.

- [ ] **Step 3: Implement layouts.go**

`internal/drivers/kitty/layouts.go`:
```go
package kitty

import "fmt"

// DefaultLayout is what kitty driver uses when layout is unspecified
// and a tab has multiple panes.
const DefaultLayout = "splits"

var validLayouts = map[string]bool{
    "splits":     true,
    "tall":       true,
    "fat":        true,
    "grid":       true,
    "horizontal": true,
    "vertical":   true,
    "stack":      true,
}

// IsValidLayout reports whether name is a kitty layout (or empty for default).
func IsValidLayout(name string) bool {
    if name == "" {
        return true
    }
    return validLayouts[name]
}

// ValidateLayout returns nil for valid layouts, an error otherwise.
func ValidateLayout(name string) error {
    if IsValidLayout(name) {
        return nil
    }
    return fmt.Errorf("kitty: %q is not a valid layout (valid: splits, tall, fat, grid, horizontal, vertical, stack)", name)
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS for 5 layout tests.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/layouts.go internal/drivers/kitty/layouts_test.go
git commit -m "kitty: layout name validator"
```

---

## Task 6: kitty match expression builder

**Files:**
- Create: `internal/drivers/kitty/match.go`
- Create: `internal/drivers/kitty/match_test.go`

- [ ] **Step 1: Write failing tests**

`internal/drivers/kitty/match_test.go`:
```go
package kitty

import "testing"

func TestMatchTabTitleExact(t *testing.T) {
    got := MatchTabTitle("demo:dev")
    want := `tab_title:^demo\:dev$`
    if got != want {
        t.Errorf("got %q, want %q", got, want)
    }
}

func TestMatchTabTitlePrefix(t *testing.T) {
    got := MatchTabTitlePrefix("demo")
    want := `tab_title:^demo\:.*$`
    if got != want {
        t.Errorf("got %q, want %q", got, want)
    }
}

func TestMatchWindowTitleExact(t *testing.T) {
    got := MatchWindowTitle("server")
    want := `title:^server$`
    if got != want {
        t.Errorf("got %q, want %q", got, want)
    }
}

func TestMatchEscapesRegex(t *testing.T) {
    got := MatchTabTitle("a.b+c")
    want := `tab_title:^a\.b\+c$`
    if got != want {
        t.Errorf("got %q, want %q", got, want)
    }
}

func TestProjectTabTitle(t *testing.T) {
    got := ProjectTabTitle("demo", "dev")
    if got != "demo:dev" {
        t.Errorf("got %q, want demo:dev", got)
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/... -run Match
```
Expected: build failure.

- [ ] **Step 3: Implement match.go**

`internal/drivers/kitty/match.go`:
```go
package kitty

import "regexp"

// ProjectTabTitle returns the canonical "<project>:<tab>" tab title.
func ProjectTabTitle(project, tab string) string {
    return project + ":" + tab
}

// MatchTabTitle returns a kitten --match expression for an exact tab title.
func MatchTabTitle(s string) string {
    return "tab_title:^" + regexp.QuoteMeta(s) + "$"
}

// MatchTabTitlePrefix returns a --match for any tab whose title is "<prefix>:*".
func MatchTabTitlePrefix(prefix string) string {
    return "tab_title:^" + regexp.QuoteMeta(prefix+":") + ".*$"
}

// MatchWindowTitle returns a --match expression for a kitty window (pane) title.
func MatchWindowTitle(s string) string {
    return "title:^" + regexp.QuoteMeta(s) + "$"
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS for 5 match tests.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/match.go internal/drivers/kitty/match_test.go
git commit -m "kitty: --match expression builders (regex-escaped)"
```

---

## Task 7: kitty session.go — leaf tab BuildCommands

**Files:**
- Create: `internal/drivers/kitty/session.go`
- Create: `internal/drivers/kitty/session_test.go`

- [ ] **Step 1: Write failing tests**

`internal/drivers/kitty/session_test.go`:
```go
package kitty

import (
    "strings"
    "testing"

    "github.com/ijcd/sesh/internal/spec"
)

func TestBuildCommands_LeafTabNoCmd(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "shell"}},
    }
    cmds, err := BuildCommands(p)
    if err != nil {
        t.Fatal(err)
    }
    if len(cmds) < 1 {
        t.Fatal("expected at least 1 cmd")
    }
    first := cmds[0]
    if !strings.Contains(first, "launch --type=tab") {
        t.Errorf("first cmd missing launch --type=tab: %s", first)
    }
    if !strings.Contains(first, "--tab-title='demo:shell'") {
        t.Errorf("first cmd missing tab title: %s", first)
    }
    if !strings.Contains(first, "--cwd='/tmp'") {
        t.Errorf("first cmd missing cwd: %s", first)
    }
}

func TestBuildCommands_LeafTabWithCmd_UsesHoldAndShellWrap(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "claude", Cmd: "claude --continue"}},
    }
    cmds, err := BuildCommands(p)
    if err != nil {
        t.Fatal(err)
    }
    first := cmds[0]
    if !strings.Contains(first, "--hold") {
        t.Errorf("expected --hold, got: %s", first)
    }
    if !strings.Contains(first, "-- /bin/sh -c 'claude --continue'") {
        t.Errorf("cmd should be sh -c wrapped: %s", first)
    }
}

func TestBuildCommands_CmdWithShellMetacharacters(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "x", Cmd: "echo \"hello world\" && tail -f log"}},
    }
    cmds, _ := BuildCommands(p)
    // The cmd must arrive intact inside the sh -c quoting; double-quotes
    // and && both must survive.
    if !strings.Contains(cmds[0], `'echo "hello world" && tail -f log'`) {
        t.Errorf("cmd not properly quoted: %s", cmds[0])
    }
}

func TestBuildCommands_FocusFirstTabByDefault(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{
            {Title: "a"}, {Title: "b"},
        },
    }
    cmds, err := BuildCommands(p)
    if err != nil {
        t.Fatal(err)
    }
    last := cmds[len(cmds)-1]
    if !strings.Contains(last, "focus-tab") || !strings.Contains(last, "demo:a") {
        t.Errorf("last cmd should focus first tab 'a': %s", last)
    }
}

func TestBuildCommands_StartupWindowOverridesFocus(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        StartupWindow: "b",
        Tabs: []spec.Tab{
            {Title: "a"}, {Title: "b"},
        },
    }
    cmds, _ := BuildCommands(p)
    last := cmds[len(cmds)-1]
    if !strings.Contains(last, "demo:b") {
        t.Errorf("focus should target b: %s", last)
    }
}

func TestBuildCommands_ErrorOnNoTabs(t *testing.T) {
    p := &spec.Project{Name: "demo", Driver: "kitty", Cwd: "/tmp"}
    _, err := BuildCommands(p)
    if err == nil {
        t.Fatal("expected error on no tabs")
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/... -run BuildCommands
```
Expected: build failure.

- [ ] **Step 3: Implement session.go (leaf-only support; multi-pane added in T8)**

`internal/drivers/kitty/session.go`:
```go
package kitty

import (
    "fmt"
    "strings"

    "github.com/ijcd/sesh/internal/spec"
)

// BuildCommands returns the exact kitten invocations the driver would run.
// Each entry begins with "kitten" (the runner adds @ --to <socket>).
func BuildCommands(p *spec.Project) ([]string, error) {
    if len(p.Tabs) == 0 {
        return nil, fmt.Errorf("project %q has no tabs", p.Name)
    }
    var cmds []string
    for _, tab := range p.Tabs {
        cmds = append(cmds, buildTab(p, tab)...)
    }
    focusTab := p.StartupWindow
    if focusTab == "" {
        focusTab = p.Tabs[0].Title
    }
    cmds = append(cmds, fmt.Sprintf(
        "kitten focus-tab --match %s",
        shellQuote(MatchTabTitle(ProjectTabTitle(p.Name, focusTab))),
    ))
    return cmds, nil
}

func buildTab(p *spec.Project, tab spec.Tab) []string {
    title := ProjectTabTitle(p.Name, tab.Title)
    cwd := tab.Cwd
    if cwd == "" {
        cwd = p.Cwd
    }
    var sb strings.Builder
    sb.WriteString("kitten launch --type=tab")
    sb.WriteString(" --tab-title=" + shellQuote(title))
    sb.WriteString(" --cwd=" + shellQuote(cwd))
    if tab.Cmd != "" {
        // Wrap the user cmd in sh -c so compound commands (`&&`, pipes,
        // redirection) and quoted arguments work. Direct exec via -- argv
        // would split on whitespace and not interpret shell syntax.
        sb.WriteString(" --hold -- /bin/sh -c " + shellQuote(tab.Cmd))
    }
    return []string{sb.String()}
}

// shellQuote wraps s in single quotes; embedded single quotes are escaped as '\''.
func shellQuote(s string) string {
    return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS for 5 BuildCommands leaf-tab tests + earlier ones.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/session.go internal/drivers/kitty/session_test.go
git commit -m "kitty: BuildCommands for leaf tabs (--type=tab, --hold, focus-tab)"
```

---

## Task 8: kitty session.go — multi-pane (kitty/kitty)

**Files:**
- Modify: `internal/drivers/kitty/session.go`
- Modify: `internal/drivers/kitty/session_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/drivers/kitty/session_test.go`:
```go
func TestBuildCommands_MultiPaneTab(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "dev", Driver: "kitty",
            Panes: []spec.Pane{
                {Title: "p1", Cmd: "x"},
                {Title: "p2", Cmd: "y"},
                {Title: "p3", Cmd: "z"},
            }}},
    }
    cmds, err := BuildCommands(p)
    if err != nil {
        t.Fatal(err)
    }
    joined := strings.Join(cmds, "\n")

    // Tab launched with first pane's title set
    if !strings.Contains(joined, `--type=tab`) {
        t.Errorf("missing tab launch: %s", joined)
    }
    // First pane: send command via --hold + -- on tab launch (no separate split-window for pane 1)
    if !strings.Contains(joined, `-- x`) {
        t.Errorf("first pane cmd should be on the tab launch: %s", joined)
    }
    // First pane window-title must be set after launch
    if !strings.Contains(joined, `set-window-title --match tab_title:^demo\:dev$`) {
        t.Errorf("first pane title not set: %s", joined)
    }

    // Second + third pane: split-window via launch --type=window
    splits := 0
    for _, c := range cmds {
        if strings.Contains(c, "launch --type=window") {
            splits++
        }
    }
    if splits != 2 {
        t.Errorf("expected 2 split-window cmds for 3 panes, got %d", splits)
    }

    // Layout default = splits when panes present
    foundLayout := false
    for _, c := range cmds {
        if strings.Contains(c, "goto-layout") && strings.Contains(c, "splits") {
            foundLayout = true
        }
    }
    if !foundLayout {
        t.Errorf("expected goto-layout splits: %s", joined)
    }
}

func TestBuildCommands_MultiPaneTabRespectsLayout(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "dev", Driver: "kitty", Layout: "tall",
            Panes: []spec.Pane{{Title: "p1", Cmd: "x"}, {Title: "p2", Cmd: "y"}}}},
    }
    cmds, _ := BuildCommands(p)
    found := false
    for _, c := range cmds {
        if strings.Contains(c, "goto-layout") && strings.Contains(c, "tall") {
            found = true
        }
    }
    if !found {
        t.Errorf("expected goto-layout tall, got %s", strings.Join(cmds, "\n"))
    }
}

func TestBuildCommands_PanesUseLocationHsplit(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "dev", Driver: "kitty",
            Panes: []spec.Pane{{Title: "p1", Cmd: "x"}, {Title: "p2", Cmd: "y"}}}},
    }
    cmds, _ := BuildCommands(p)
    for _, c := range cmds {
        if strings.Contains(c, "launch --type=window") && !strings.Contains(c, "--location=hsplit") {
            t.Errorf("split-window cmd missing --location=hsplit: %s", c)
        }
    }
}

func TestBuildCommands_PaneCwdRelativeToTabCwd(t *testing.T) {
    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/home/me",
        Tabs: []spec.Tab{{Title: "dev", Driver: "kitty", Cwd: "src",
            Panes: []spec.Pane{
                {Title: "p1", Cmd: "x", Cwd: "lib"},
            }}},
    }
    cmds, _ := BuildCommands(p)
    for _, c := range cmds {
        if strings.Contains(c, "launch --type=window") {
            if !strings.Contains(c, "--cwd='/home/me/src/lib'") {
                t.Errorf("pane cwd should be joined: %s", c)
            }
        }
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/...
```
Expected: 4 new tests fail.

- [ ] **Step 3: Implement multi-pane support**

Replace `buildTab` in `internal/drivers/kitty/session.go`:
```go
func buildTab(p *spec.Project, tab spec.Tab) []string {
    title := ProjectTabTitle(p.Name, tab.Title)
    tabCwd := resolveCwd(tab.Cwd, p.Cwd)
    var cmds []string

    // Launch the tab. First pane (if multi-pane) is the existing tab window.
    var sb strings.Builder
    sb.WriteString("kitten launch --type=tab")
    sb.WriteString(" --tab-title=" + shellQuote(title))
    sb.WriteString(" --cwd=" + shellQuote(tabCwd))
    var firstCmd string
    if len(tab.Panes) > 0 {
        firstCmd = tab.Panes[0].Cmd
    } else {
        firstCmd = tab.Cmd
    }
    if firstCmd != "" {
        sb.WriteString(" --hold -- /bin/sh -c " + shellQuote(firstCmd))
    }
    cmds = append(cmds, sb.String())

    // Set the first pane's window title (if it was a pane).
    if len(tab.Panes) > 0 {
        cmds = append(cmds, fmt.Sprintf(
            "kitten set-window-title --match %s %s",
            shellQuote(MatchTabTitle(title)),
            shellQuote(tab.Panes[0].Title),
        ))
    }

    // Subsequent panes: split-window via --type=window.
    for _, pane := range pickAfter(tab.Panes, 1) {
        paneCwd := resolveCwd(pane.Cwd, tabCwd)
        var psb strings.Builder
        psb.WriteString("kitten launch --type=window --location=hsplit")
        psb.WriteString(" --match " + shellQuote(MatchTabTitle(title)))
        psb.WriteString(" --window-title=" + shellQuote(pane.Title))
        psb.WriteString(" --cwd=" + shellQuote(paneCwd))
        if pane.Cmd != "" {
            psb.WriteString(" --hold -- /bin/sh -c " + shellQuote(pane.Cmd))
        }
        cmds = append(cmds, psb.String())
    }

    // Apply layout for multi-pane tabs.
    if len(tab.Panes) > 1 {
        layout := tab.Layout
        if layout == "" {
            layout = DefaultLayout
        }
        cmds = append(cmds, fmt.Sprintf(
            "kitten goto-layout --match %s %s",
            shellQuote(MatchTabTitle(title)), shellQuote(layout),
        ))
    }
    return cmds
}

func pickAfter[T any](s []T, n int) []T {
    if n >= len(s) {
        return nil
    }
    return s[n:]
}

// resolveCwd: absolute child kept as-is; relative child joined with parent;
// empty child inherits parent.
func resolveCwd(child, parent string) string {
    if child == "" {
        return parent
    }
    if filepath.IsAbs(child) {
        return child
    }
    return filepath.Join(parent, child)
}
```

Add import `"path/filepath"`.

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS for all 9 BuildCommands tests.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/session.go internal/drivers/kitty/session_test.go
git commit -m "kitty: multi-pane tabs via launch --type=window + goto-layout"
```

---

## Task 9: kitty Driver — Name, Up, DryRun

**Files:**
- Create: `internal/drivers/kitty/driver.go`
- Create: `internal/drivers/kitty/driver_test.go`

- [ ] **Step 1: Write failing tests**

`internal/drivers/kitty/driver_test.go`:
```go
package kitty

import (
    "context"
    "errors"
    "strings"
    "testing"

    "github.com/ijcd/sesh/internal/spec"
)

func TestDriver_Name(t *testing.T) {
    d := New()
    if d.Name() != "kitty" {
        t.Errorf("Name = %q", d.Name())
    }
}

func TestDriver_Up_RunsBuiltCommands(t *testing.T) {
    fr := &fakeRunner{}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p := &spec.Project{Name: "x", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "shell"}}}
    if err := d.Up(context.Background(), p); err != nil {
        t.Fatal(err)
    }
    if len(fr.runs) == 0 {
        t.Fatal("no kitten cmds run")
    }
    if !strings.Contains(fr.runs[0], "launch --type=tab") {
        t.Errorf("first run not launch --type=tab: %q", fr.runs[0])
    }
}

func TestDriver_Up_FailsWithoutKittyListenOn(t *testing.T) {
    fr := &fakeRunner{}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "")
    p := &spec.Project{Name: "x", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "shell"}}}
    err := d.Up(context.Background(), p)
    if err == nil {
        t.Fatal("expected error")
    }
    if !strings.Contains(err.Error(), "KITTY_LISTEN_ON") {
        t.Errorf("err should mention KITTY_LISTEN_ON: %v", err)
    }
}

func TestDriver_Up_RunnerErrorAborts(t *testing.T) {
    fr := &fakeRunner{runErr: errors.New("boom")}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p := &spec.Project{Name: "x", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "shell"}}}
    err := d.Up(context.Background(), p)
    if err == nil {
        t.Fatal("expected error")
    }
}

func TestDriver_DryRun_NeedsNoSocket(t *testing.T) {
    d := New()
    t.Setenv("KITTY_LISTEN_ON", "")
    p := &spec.Project{Name: "x", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{{Title: "shell"}}}
    cmds, err := d.DryRun(p)
    if err != nil {
        t.Fatal(err)
    }
    if len(cmds) == 0 {
        t.Errorf("DryRun returned no commands")
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/... -run Driver
```
Expected: build failure (`Driver`, `New`, `newWith` undefined).

- [ ] **Step 3: Implement driver.go (subset for this task)**

`internal/drivers/kitty/driver.go`:
```go
package kitty

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/ijcd/sesh/internal/drivers"
    "github.com/ijcd/sesh/internal/spec"
)

type Driver struct {
    r Runner
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
```

This will fail to compile because the interface requires Down/Status/Capture/Validate/AttachCommand. Add stubs:

```go
func (d *Driver) Down(ctx context.Context, name string) error {
    return fmt.Errorf("kitty: Down not yet implemented")
}
func (d *Driver) Status(ctx context.Context, name string) (drivers.Status, error) {
    return drivers.StatusUnknown, nil
}
func (d *Driver) Capture(ctx context.Context) (*spec.Project, error) {
    return nil, nil
}
func (d *Driver) Validate(p *spec.Project) []error { return nil }
func (d *Driver) AttachCommand(p *spec.Project) (string, error) {
    return "", fmt.Errorf("kitty: AttachCommand not supported (kitty has no detached sessions)")
}
```

These stubs are fleshed out in T10–T13.

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS for the 5 driver tests + earlier tests.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/driver.go internal/drivers/kitty/driver_test.go
git commit -m "kitty: Driver Name/Up/DryRun + interface stubs"
```

---

## Task 10: kitty Driver — Status (title-prefix scan)

**Files:**
- Modify: `internal/drivers/kitty/driver.go`
- Modify: `internal/drivers/kitty/driver_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/drivers/kitty/driver_test.go`:
```go
func TestDriver_Status_Exists(t *testing.T) {
    fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [
        {"title": "demo:shell"}, {"title": "other:dev"}
      ]}
    ]`}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    s, err := d.Status(context.Background(), "demo")
    if err != nil {
        t.Fatal(err)
    }
    if s != drivers.StatusExists {
        t.Errorf("Status = %q, want exists", s)
    }
}

func TestDriver_Status_NotExists(t *testing.T) {
    fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [{"title": "other:thing"}]}
    ]`}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    s, err := d.Status(context.Background(), "demo")
    if err != nil {
        t.Fatal(err)
    }
    if s != drivers.StatusNotExists {
        t.Errorf("Status = %q, want not_exists", s)
    }
}

func TestDriver_Status_RunnerError(t *testing.T) {
    fr := &fakeRunner{captureErr: errors.New("boom")}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    _, err := d.Status(context.Background(), "demo")
    if err == nil {
        t.Fatal("expected error")
    }
}
```

Add the `drivers` import to driver_test.go.

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/... -run Status
```
Expected: FAIL — Status stub returns Unknown.

- [ ] **Step 3: Implement Status**

Replace the Status stub in `internal/drivers/kitty/driver.go`:
```go
import (
    // ... existing
    "encoding/json"
)

type kittyOSWindow struct {
    IsFocused bool       `json:"is_focused"`
    Tabs      []kittyTab `json:"tabs"`
}
type kittyTab struct {
    Title   string         `json:"title"`
    Windows []kittyWindow  `json:"windows"`
}
type kittyWindow struct {
    IsFocused           bool                  `json:"is_focused"`
    Cwd                 string                `json:"cwd"`
    ForegroundProcesses []kittyForegroundProc `json:"foreground_processes"`
}
type kittyForegroundProc struct {
    Cmdline []string `json:"cmdline"`
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
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/driver.go internal/drivers/kitty/driver_test.go
git commit -m "kitty: Status via tab-title-prefix scan of kitten ls"
```

---

## Task 11: kitty Driver — Down

**Files:**
- Modify: `internal/drivers/kitty/driver.go`
- Modify: `internal/drivers/kitty/driver_test.go`

- [ ] **Step 1: Write failing tests**

Append to `driver_test.go`:
```go
func TestDriver_Down_ClosesProjectTabs(t *testing.T) {
    fr := &fakeRunner{}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    if err := d.Down(context.Background(), "demo"); err != nil {
        t.Fatal(err)
    }
    if len(fr.runs) != 1 {
        t.Fatalf("expected 1 run, got %d: %v", len(fr.runs), fr.runs)
    }
    if !strings.Contains(fr.runs[0], "close-tab") {
        t.Errorf("expected close-tab: %s", fr.runs[0])
    }
    if !strings.Contains(fr.runs[0], "tab_title:^demo\\:.*$") {
        t.Errorf("expected prefix match for demo: %s", fr.runs[0])
    }
}

func TestDriver_Down_RunnerErrorSurfaced(t *testing.T) {
    fr := &fakeRunner{runErr: errors.New("nope")}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    if err := d.Down(context.Background(), "demo"); err == nil {
        t.Fatal("expected error")
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/... -run Down
```
Expected: FAIL.

- [ ] **Step 3: Implement Down**

Replace the Down stub:
```go
func (d *Driver) Down(ctx context.Context, name string) error {
    r, err := d.runner()
    if err != nil {
        return err
    }
    return r.Run(ctx, "close-tab", "--match", MatchTabTitlePrefix(name))
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/driver.go internal/drivers/kitty/driver_test.go
git commit -m "kitty: Down closes all tabs matching <project>: prefix"
```

---

## Task 12: kitty Driver — Validate

**Files:**
- Modify: `internal/drivers/kitty/driver.go`
- Modify: `internal/drivers/kitty/driver_test.go`

- [ ] **Step 1: Write failing tests**

Append to `driver_test.go`:
```go
func TestDriver_Validate_OK(t *testing.T) {
    d := New()
    p := &spec.Project{Driver: "kitty", Tabs: []spec.Tab{{Title: "shell"}}}
    if errs := d.Validate(p); len(errs) > 0 {
        t.Errorf("unexpected errors: %v", errs)
    }
}

func TestDriver_Validate_BadLayout(t *testing.T) {
    d := New()
    p := &spec.Project{Driver: "kitty", Tabs: []spec.Tab{{
        Title: "x", Layout: "main-vertical",
        Panes: []spec.Pane{{Title: "p", Cmd: "y"}},
    }}}
    errs := d.Validate(p)
    if len(errs) == 0 {
        t.Fatal("expected layout error")
    }
    if !strings.Contains(errs[0].Error(), "main-vertical") {
        t.Errorf("error should mention bad layout: %v", errs[0])
    }
}

func TestDriver_Validate_TabTitleWithColon(t *testing.T) {
    d := New()
    p := &spec.Project{Driver: "kitty", Tabs: []spec.Tab{{Title: "with:colon"}}}
    errs := d.Validate(p)
    if len(errs) == 0 {
        t.Fatal("expected error for colon in tab title")
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/... -run Validate
```
Expected: FAIL.

- [ ] **Step 3: Implement Validate**

Replace the Validate stub:
```go
func (d *Driver) Validate(p *spec.Project) []error {
    var errs []error
    if _, err := kittenPath(); err != nil {
        errs = append(errs, fmt.Errorf("kitty driver: %w", err))
    }
    for i, t := range p.Tabs {
        if strings.Contains(t.Title, ":") {
            errs = append(errs, fmt.Errorf("tabs[%d].title: must not contain ':' (sesh uses '<project>:<tab>' tagging)", i))
        }
        if err := ValidateLayout(t.Layout); err != nil {
            errs = append(errs, fmt.Errorf("tabs[%d].layout: %w", i, err))
        }
    }
    return errs
}
```

Note: missing-kitten error will fire on dev machines without kitten. Tests are run on machines with kitten available, so the OK tests should still pass. If CI fails because kitten is absent, the validate function should be conditioned: only error if kitten is missing AND a real Up would happen. For simplicity in v0.2, accept the limitation and skip these tests on machines without kitten:

Add at top of relevant test funcs:
```go
if _, err := exec.LookPath("kitten"); err != nil {
    t.Skip("kitten not installed")
}
```

(Apply that skip to TestDriver_Validate_OK only; the BadLayout and ColonTitle tests don't require kitten because the layout/colon errors fire before kitten check returns, and even if it returns an extra error, the test asserts at least one matching error.)

Actually, we want validate's missing-kitten to be the FIRST error consulted, but the test asserts on substring match of `errs[0]`. To make tests robust, let's not append the kitten error before the others:

Re-implement Validate with kitten check LAST so other validation errors come first:
```go
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
    if _, err := kittenPath(); err != nil {
        errs = append(errs, fmt.Errorf("kitty driver: %w", err))
    }
    return errs
}
```

Tests use Contains across all errs, not just errs[0]:
```go
func anyErrorContains(errs []error, substr string) bool {
    for _, e := range errs {
        if strings.Contains(e.Error(), substr) {
            return true
        }
    }
    return false
}
```

Update tests to use anyErrorContains.

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/driver.go internal/drivers/kitty/driver_test.go
git commit -m "kitty: Validate (layout name, no-colon tab title, kitten present)"
```

---

## Task 13: kitty Driver — AttachCommand error case

**Files:**
- Modify: `internal/drivers/kitty/driver.go` (already has the stub from T9)
- Modify: `internal/drivers/kitty/driver_test.go`

- [ ] **Step 1: Write failing test**

Append to `driver_test.go`:
```go
func TestDriver_AttachCommand_NotSupported(t *testing.T) {
    d := New()
    p := &spec.Project{Name: "x", Driver: "kitty"}
    _, err := d.AttachCommand(p)
    if err == nil {
        t.Fatal("expected error — kitty has no attach")
    }
    if !strings.Contains(err.Error(), "kitty") {
        t.Errorf("error should mention kitty: %v", err)
    }
}
```

- [ ] **Step 2: Run, confirm pass (stub already exists)**

```bash
go test ./internal/drivers/kitty/... -run AttachCommand
```
Expected: PASS (stub from T9 already returns the error).

- [ ] **Step 3: Commit**

```bash
git add internal/drivers/kitty/driver_test.go
git commit -m "kitty: test AttachCommand error message"
```

---

## Task 14: kitty Driver — Capture

**Files:**
- Create: `internal/drivers/kitty/capture.go`
- Create: `internal/drivers/kitty/capture_test.go`

- [ ] **Step 1: Write failing tests**

`internal/drivers/kitty/capture_test.go`:
```go
package kitty

import (
    "context"
    "testing"
)

func TestCapture_NoOSWindows(t *testing.T) {
    fr := &fakeRunner{captureOut: `[]`}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p, err := d.Capture(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if p != nil {
        t.Errorf("expected nil project, got %+v", p)
    }
}

func TestCapture_SingleTabSinglePane(t *testing.T) {
    fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [
        {"title": "demo:shell", "windows": [
          {"is_focused": true, "cwd": "/tmp",
           "foreground_processes": [{"cmdline": ["zsh"]}]}
        ]}
      ]}
    ]`}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p, err := d.Capture(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if p == nil {
        t.Fatal("expected non-nil project")
    }
    if p.Driver != "kitty" || p.Cwd != "/tmp" {
        t.Errorf("got %+v", p)
    }
    if len(p.Tabs) != 1 || p.Tabs[0].Title != "shell" {
        t.Errorf("expected one tab 'shell', got %+v", p.Tabs)
    }
    if p.Tabs[0].Cmd != "" {
        t.Errorf("zsh should normalize to empty, got %q", p.Tabs[0].Cmd)
    }
}

func TestCapture_MultiTabMultiPane(t *testing.T) {
    fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [
        {"title": "demo:claude", "windows": [
          {"cwd": "/tmp", "foreground_processes": [{"cmdline": ["claude", "--continue"]}]}
        ]},
        {"title": "demo:dev", "windows": [
          {"cwd": "/tmp/x", "foreground_processes": [{"cmdline": ["overmind", "start"]}]},
          {"cwd": "/tmp/x", "foreground_processes": [{"cmdline": ["iex", "-S", "mix"]}]}
        ]}
      ]}
    ]`}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p, _ := d.Capture(context.Background())
    if p == nil || len(p.Tabs) != 2 {
        t.Fatalf("expected 2 tabs, got %+v", p)
    }
    // Tab 0: single pane, Cmd populated
    if p.Tabs[0].Title != "claude" || p.Tabs[0].Cmd == "" {
        t.Errorf("tab 0 wrong: %+v", p.Tabs[0])
    }
    // Tab 1: multi-pane
    if p.Tabs[1].Title != "dev" || len(p.Tabs[1].Panes) != 2 {
        t.Errorf("tab 1 wrong: %+v", p.Tabs[1])
    }
    if p.Tabs[1].Panes[0].Title != "p1" || p.Tabs[1].Panes[1].Title != "p2" {
        t.Errorf("auto-titles wrong: %+v", p.Tabs[1].Panes)
    }
}

func TestCapture_OvermindNormalization(t *testing.T) {
    fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [
        {"title": "demo:dev", "windows": [
          {"cwd": "/x",
           "foreground_processes": [{"cmdline": ["tmux", "-L", "overmind-abc", "attach"]}]}
        ]}
      ]}
    ]`}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p, _ := d.Capture(context.Background())
    if p.Tabs[0].Cmd != "overmind start" {
        t.Errorf("expected overmind normalization, got %q", p.Tabs[0].Cmd)
    }
}

func TestCapture_PrefersFocusedOSWindow(t *testing.T) {
    fr := &fakeRunner{captureOut: `[
      {"is_focused": false, "tabs": [{"title": "ignore:me"}]},
      {"is_focused": true, "tabs": [
        {"title": "demo:shell", "windows": [{"cwd": "/tmp"}]}
      ]}
    ]`}
    d := newWith(fr)
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p, _ := d.Capture(context.Background())
    if p == nil || p.Tabs[0].Title != "shell" {
        t.Errorf("expected focused-window's tab, got %+v", p)
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/... -run Capture
```
Expected: FAIL — Capture stub returns nil.

- [ ] **Step 3: Implement Capture**

`internal/drivers/kitty/capture.go`:
```go
package kitty

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"

    "github.com/ijcd/sesh/internal/spec"
)

// Capture parses `kitten ls` and produces a draft *spec.Project for the
// focused OS window. Returns (nil, nil) if no OS windows are present.
func (d *Driver) Capture(ctx context.Context) (*spec.Project, error) {
    r, err := d.runner()
    if err != nil {
        return nil, err
    }
    out, err := r.RunCapture(ctx, "ls")
    if err != nil {
        return nil, fmt.Errorf("kitten ls: %w", err)
    }
    var wins []kittyOSWindow
    if err := json.Unmarshal([]byte(out), &wins); err != nil {
        return nil, fmt.Errorf("parse kitten ls: %w", err)
    }
    if len(wins) == 0 {
        return nil, nil
    }
    win := pickFocused(wins)

    p := &spec.Project{Driver: "kitty"}
    cwds := []string{}
    for _, t := range win.Tabs {
        title := stripPrefix(t.Title)
        tab := spec.Tab{Title: title}
        cmds := []string{}
        for _, w := range t.Windows {
            if w.Cwd != "" {
                cwds = append(cwds, w.Cwd)
            }
            if len(w.ForegroundProcesses) > 0 {
                if c := normalizeCmdline(w.ForegroundProcesses[0].Cmdline); c != "" {
                    cmds = append(cmds, c)
                }
            } else {
                cmds = append(cmds, "")
            }
        }
        switch {
        case len(t.Windows) == 0:
            // empty tab; no cmd
        case len(t.Windows) == 1:
            if len(cmds) > 0 {
                tab.Cmd = cmds[0]
            }
        default:
            tab.Driver = "kitty"
            for i := range t.Windows {
                cmd := ""
                if i < len(cmds) {
                    cmd = cmds[i]
                }
                tab.Panes = append(tab.Panes, spec.Pane{
                    Title: fmt.Sprintf("p%d", i+1), Cmd: cmd,
                })
            }
        }
        p.Tabs = append(p.Tabs, tab)
    }
    p.Cwd = mostCommonCwd(cwds)
    return p, nil
}

func pickFocused(wins []kittyOSWindow) kittyOSWindow {
    for _, w := range wins {
        if w.IsFocused {
            return w
        }
    }
    return wins[0]
}

func stripPrefix(title string) string {
    i := strings.IndexByte(title, ':')
    if i < 0 {
        return title
    }
    return title[i+1:]
}

func mostCommonCwd(cwds []string) string {
    if len(cwds) == 0 {
        if home, err := os.UserHomeDir(); err == nil {
            return home
        }
        return ""
    }
    counts := map[string]int{}
    var best string
    for _, c := range cwds {
        counts[c]++
        if counts[c] > counts[best] {
            best = c
        }
    }
    return best
}

func normalizeCmdline(cmdline []string) string {
    if len(cmdline) == 0 {
        return ""
    }
    bin := cmdline[0]
    // strip leading dash for login shells
    if strings.HasPrefix(bin, "-") {
        bin = bin[1:]
    }
    base := lastPathSegment(bin)
    switch base {
    case "zsh", "bash", "fish", "sh":
        return ""
    case "tmux":
        for _, a := range cmdline {
            if strings.HasPrefix(a, "overmind-") {
                return "overmind start"
            }
        }
    }
    return strings.Join(cmdline, " ")
}

func lastPathSegment(s string) string {
    if i := strings.LastIndexByte(s, '/'); i >= 0 {
        return s[i+1:]
    }
    return s
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/capture.go internal/drivers/kitty/capture_test.go
git commit -m "kitty: Capture (kitten ls → draft spec.Project)"
```

---

## Task 15: Engine cross-driver dispatch (kitty/tmux)

**Files:**
- Modify: `internal/engine/up.go`
- Create: `internal/engine/cross_driver_test.go`

- [ ] **Step 1: Write failing test**

`internal/engine/cross_driver_test.go`:
```go
package engine

import (
    "context"
    "testing"

    "github.com/ijcd/sesh/internal/drivers/mock"
    "github.com/ijcd/sesh/internal/spec"
)

func TestUp_CrossDriverDispatch(t *testing.T) {
    tmux := mock.New("tmux")
    tmux.AttachCommandVal = "tmux attach -t demo-dev"
    kitty := mock.New("kitty")
    e := New()
    e.Register(tmux)
    e.Register(kitty)

    p := &spec.Project{
        Name: "demo", Driver: "kitty", Cwd: "/tmp",
        Tabs: []spec.Tab{
            {Title: "claude", Cmd: "claude --continue"},
            {Title: "dev", Driver: "tmux", Panes: []spec.Pane{
                {Title: "server", Cmd: "overmind start"},
                {Title: "repl", Cmd: "iex -S mix"},
            }},
        },
    }

    if err := e.Up(context.Background(), p, false); err != nil {
        t.Fatal(err)
    }

    // tmux driver got an inner project for the dev tab
    if len(tmux.UpCalls) != 1 {
        t.Fatalf("expected tmux.Up called once, got %d", len(tmux.UpCalls))
    }
    inner := tmux.UpCalls[0]
    if inner.Driver != "tmux" || len(inner.Tabs) != 1 || inner.Tabs[0].Title != "dev" {
        t.Errorf("inner project wrong: %+v", inner)
    }

    // kitty driver got the outer project, but tab 'dev' should now be a leaf
    // with cmd = "tmux attach -t demo-dev" and no Panes.
    if len(kitty.UpCalls) != 1 {
        t.Fatalf("expected kitty.Up called once, got %d", len(kitty.UpCalls))
    }
    outer := kitty.UpCalls[0]
    var devTab *spec.Tab
    for i := range outer.Tabs {
        if outer.Tabs[i].Title == "dev" {
            devTab = &outer.Tabs[i]
        }
    }
    if devTab == nil {
        t.Fatal("dev tab missing from outer")
    }
    if devTab.Cmd != "tmux attach -t demo-dev" {
        t.Errorf("dev.Cmd = %q, want attach string", devTab.Cmd)
    }
    if len(devTab.Panes) != 0 {
        t.Errorf("dev.Panes should be empty, got %v", devTab.Panes)
    }
    if devTab.Driver != "" {
        t.Errorf("dev.Driver should be cleared (now leaf), got %q", devTab.Driver)
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/engine/... -run CrossDriver
```
Expected: FAIL — engine.Up doesn't do cross-driver dispatch yet.

- [ ] **Step 3: Implement cross-driver branch in engine.Up**

Edit `internal/engine/up.go`. Add helper above `Up`:

```go
// transformCrossDriverTabs splits each tab whose driver differs from the project
// driver and has panes. The inner tabs are dispatched to their child driver
// (creating that driver's container) and the outer project is mutated so each
// such tab becomes a leaf cmd that attaches to the inner container.
//
// Returns the modified project (a deep-ish copy of p) and any error.
func (e *Engine) transformCrossDriverTabs(ctx context.Context, p *spec.Project) (*spec.Project, error) {
    out := *p
    out.Tabs = make([]spec.Tab, len(p.Tabs))
    copy(out.Tabs, p.Tabs)

    for i := range out.Tabs {
        t := &out.Tabs[i]
        childDrv := t.Driver
        if childDrv == "" || childDrv == p.Driver || len(t.Panes) == 0 {
            continue
        }
        // Cross-driver pair: dispatch inner first.
        cd, err := e.driverFor(childDrv)
        if err != nil {
            return nil, err
        }
        innerName := slugInnerName(p.Name, t.Title)
        innerTab := *t
        innerTab.Driver = "" // child handles its own driver default
        inner := &spec.Project{
            Name: innerName, Driver: childDrv, Cwd: t.Cwd,
            Tabs: []spec.Tab{innerTab},
        }
        if inner.Cwd == "" {
            inner.Cwd = p.Cwd
        }
        if err := cd.Up(ctx, inner); err != nil {
            return nil, fmt.Errorf("cross-driver up for tab %q: %w", t.Title, err)
        }
        attachCmd, err := cd.AttachCommand(inner)
        if err != nil {
            return nil, fmt.Errorf("cross-driver AttachCommand for tab %q: %w", t.Title, err)
        }
        // Replace the outer tab with a leaf cmd that attaches.
        t.Cmd = attachCmd
        t.Panes = nil
        t.Driver = ""
    }
    return &out, nil
}

func slugInnerName(project, tab string) string {
    return project + "-" + tab
}
```

Then update `Up` to call this transform before `applyPreWindow`:

```go
func (e *Engine) Up(ctx context.Context, p *spec.Project, force bool) error {
    if err := CheckContainment(p); err != nil {
        return err
    }
    if err := RunHooks(ctx, "pre", p.Hooks.Pre, p.Cwd); err != nil {
        return err
    }
    pp, err := e.transformCrossDriverTabs(ctx, p)
    if err != nil {
        return err
    }
    d, err := e.driverFor(pp.Driver)
    if err != nil {
        return err
    }
    pp = applyPreWindow(pp)
    // ... rest unchanged
}
```

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```
Expected: PASS — including new CrossDriver test, no regressions.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/
git commit -m "engine: cross-driver tab dispatch (kitty hosting tmux)"
```

---

## Task 16: Register kitty driver in CLI

**Files:**
- Modify: `cmd/sesh/root.go`

- [ ] **Step 1: Register kitty alongside tmux**

Edit `cmd/sesh/root.go`:
```go
import (
    // ... existing
    "github.com/ijcd/sesh/internal/drivers/kitty"
)

// inside newRootCmd:
e.Register(tmux.New())
e.Register(kitty.New())
```

- [ ] **Step 2: Build and verify**

```bash
go build -o sesh ./cmd/sesh
./sesh ls
go test ./...
```
Expected: clean build, tests pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/sesh/root.go
git commit -m "cli: register kitty driver alongside tmux"
```

---

## Task 17: --launch package (spawn kitty + wait for socket)

**Files:**
- Create: `internal/drivers/kitty/launch/launch.go`
- Create: `internal/drivers/kitty/launch/launch_test.go`

- [ ] **Step 1: Write failing tests**

`internal/drivers/kitty/launch/launch_test.go`:
```go
package launch

import (
    "context"
    "errors"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestWaitForSocket_AppearsInTime(t *testing.T) {
    dir := t.TempDir()
    sock := filepath.Join(dir, "test.sock")
    go func() {
        time.Sleep(50 * time.Millisecond)
        os.WriteFile(sock, []byte{}, 0o644)
    }()
    if err := WaitForSocket(context.Background(), sock, 1*time.Second); err != nil {
        t.Errorf("expected nil, got %v", err)
    }
}

func TestWaitForSocket_TimesOut(t *testing.T) {
    dir := t.TempDir()
    sock := filepath.Join(dir, "missing.sock")
    err := WaitForSocket(context.Background(), sock, 100*time.Millisecond)
    if err == nil {
        t.Fatal("expected timeout error")
    }
    if !errors.Is(err, ErrTimeout) {
        t.Errorf("expected ErrTimeout, got %v", err)
    }
}

func TestWaitForSocket_PreExisting(t *testing.T) {
    dir := t.TempDir()
    sock := filepath.Join(dir, "exists.sock")
    if err := os.WriteFile(sock, []byte{}, 0o644); err != nil {
        t.Fatal(err)
    }
    if err := WaitForSocket(context.Background(), sock, 50*time.Millisecond); err != nil {
        t.Errorf("expected pre-existing socket to succeed, got %v", err)
    }
}

func TestSocketPathFor(t *testing.T) {
    dir := t.TempDir()
    p, err := SocketPathFor("liberties", dir)
    if err != nil {
        t.Fatal(err)
    }
    want := filepath.Join(dir, "sockets", "liberties.sock")
    if p != want {
        t.Errorf("got %q, want %q", p, want)
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./internal/drivers/kitty/launch/...
```
Expected: build failure.

- [ ] **Step 3: Implement launch.go**

`internal/drivers/kitty/launch/launch.go`:
```go
// Package launch spawns a fresh kitty instance bound to a sesh-controlled
// remote-control socket. Used by the cmd layer when sesh up --launch is
// invoked outside of an existing kitty.
package launch

import (
    "context"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "syscall"
    "time"
)

var ErrTimeout = errors.New("timed out waiting for kitty socket")

// SocketPathFor returns the canonical socket path for a project under stateDir.
func SocketPathFor(project, stateDir string) (string, error) {
    dir := filepath.Join(stateDir, "sockets")
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return "", err
    }
    return filepath.Join(dir, project+".sock"), nil
}

// SpawnKitty starts a detached kitty process bound to socket. Returns the
// PID of the spawned process. Caller is responsible for waiting on the
// socket via WaitForSocket and for persisting the PID/socket in state.
func SpawnKitty(socket string) (int, error) {
    kitty, err := exec.LookPath("kitty")
    if err != nil {
        // macOS fallback
        for _, p := range []string{
            "/Applications/kitty.app/Contents/MacOS/kitty",
            "/opt/homebrew/bin/kitty",
        } {
            if _, e := os.Stat(p); e == nil {
                kitty = p
                err = nil
                break
            }
        }
        if err != nil {
            return 0, fmt.Errorf("kitty not found in PATH: %w", err)
        }
    }
    args := []string{
        "--listen-on=unix:" + socket,
        "--override", "allow_remote_control=yes",
        "--single-instance",
        "--instance-group=sesh-" + filepath.Base(socket),
    }
    cmd := exec.Command(kitty, args...)
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
    cmd.Stdin = nil
    cmd.Stdout = nil
    cmd.Stderr = nil
    if err := cmd.Start(); err != nil {
        return 0, fmt.Errorf("spawn kitty: %w", err)
    }
    // Detach: don't Wait. Process becomes child of init when sesh exits.
    return cmd.Process.Pid, nil
}

// WaitForSocket polls until socket file exists or timeout elapses.
func WaitForSocket(ctx context.Context, socket string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for {
        if _, err := os.Stat(socket); err == nil {
            return nil
        }
        if time.Now().After(deadline) {
            return fmt.Errorf("%w: %s", ErrTimeout, socket)
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(20 * time.Millisecond):
        }
    }
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/drivers/kitty/launch/...
```
Expected: PASS for the 4 unit tests (SpawnKitty isn't directly tested here — it requires real kitty; covered in integration test).

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/launch/
git commit -m "kitty/launch: spawn kitty + wait for socket (timeout-bounded)"
```

---

## Task 18: cmd/sesh/up.go --launch flag

**Files:**
- Modify: `cmd/sesh/up.go`
- Create: `cmd/sesh/launch_helper.go`
- Create: `cmd/sesh/launch_helper_test.go`

- [ ] **Step 1: Write tests for the launch helper**

`cmd/sesh/launch_helper_test.go`:
```go
package main

import (
    "testing"

    "github.com/ijcd/sesh/internal/spec"
)

func TestNeedsLaunch_InKittyNoFlag(t *testing.T) {
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p := &spec.Project{Driver: "kitty"}
    if needsLaunch(p, false) {
        t.Error("in-kitty + no flag → should not launch")
    }
}

func TestNeedsLaunch_InKittyWithFlag(t *testing.T) {
    t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
    p := &spec.Project{Driver: "kitty"}
    if needsLaunch(p, true) {
        t.Error("in-kitty + flag → should be no-op (use existing kitty)")
    }
}

func TestNeedsLaunch_NotInKittyNoFlag(t *testing.T) {
    t.Setenv("KITTY_LISTEN_ON", "")
    p := &spec.Project{Driver: "kitty"}
    if needsLaunch(p, false) {
        t.Error("not-in-kitty + no flag should error elsewhere, not launch")
    }
}

func TestNeedsLaunch_NotInKittyWithFlag(t *testing.T) {
    t.Setenv("KITTY_LISTEN_ON", "")
    p := &spec.Project{Driver: "kitty"}
    if !needsLaunch(p, true) {
        t.Error("not-in-kitty + flag → should launch")
    }
}

func TestNeedsLaunch_NonKittyDriver(t *testing.T) {
    t.Setenv("KITTY_LISTEN_ON", "")
    p := &spec.Project{Driver: "tmux"}
    if needsLaunch(p, true) {
        t.Error("tmux project → never launches kitty")
    }
}
```

- [ ] **Step 2: Run, confirm failure**

```bash
go test ./cmd/sesh/... -run NeedsLaunch
```
Expected: build failure.

- [ ] **Step 3: Implement helper**

`cmd/sesh/launch_helper.go`:
```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/ijcd/sesh/internal/drivers/kitty/launch"
    "github.com/ijcd/sesh/internal/spec"
    "github.com/ijcd/sesh/internal/state"
)

// needsLaunch reports whether the project requires sesh to spawn a fresh
// kitty before proceeding. True only when: driver is kitty, KITTY_LISTEN_ON
// is unset, AND the user passed --launch.
func needsLaunch(p *spec.Project, launchFlag bool) bool {
    if p.Driver != "kitty" {
        return false
    }
    if os.Getenv("KITTY_LISTEN_ON") != "" {
        return false
    }
    return launchFlag
}

// requiresKitty returns an error if the project needs kitty access but
// none is available.
func requiresKitty(p *spec.Project, launchFlag bool) error {
    if p.Driver != "kitty" {
        return nil
    }
    if os.Getenv("KITTY_LISTEN_ON") != "" {
        return nil
    }
    if launchFlag {
        return nil
    }
    return fmt.Errorf("kitty driver requires running inside kitty (KITTY_LISTEN_ON unset). Run again with --launch to spawn a new kitty for this project")
}

// performLaunch spawns kitty for the project, waits for the socket,
// sets KITTY_LISTEN_ON, and persists the launch state. Returns nil on
// success.
func performLaunch(ctx context.Context, p *spec.Project) error {
    stateDir, err := stateBaseDir()
    if err != nil {
        return err
    }
    sockPath, err := launch.SocketPathFor(p.Name, stateDir)
    if err != nil {
        return err
    }
    pid, err := launch.SpawnKitty(sockPath)
    if err != nil {
        return err
    }
    if err := launch.WaitForSocket(ctx, sockPath, 5*time.Second); err != nil {
        return fmt.Errorf("kitty did not become ready: %w", err)
    }
    os.Setenv("KITTY_LISTEN_ON", "unix:"+sockPath)

    statePath, err := state.DefaultPath()
    if err != nil {
        return err
    }
    s, err := state.Load(statePath)
    if err != nil {
        return err
    }
    s.Set(p.Name, state.LaunchEntry{
        Socket: sockPath, Pid: pid, LaunchedAt: time.Now(),
    })
    return s.Save(statePath)
}

func stateBaseDir() (string, error) {
    if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
        return xdg + "/sesh", nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return home + "/.local/state/sesh", nil
}
```

- [ ] **Step 4: Update cmd/sesh/up.go to add the --launch flag**

Replace `cmd/sesh/up.go` with:
```go
package main

import (
    "context"

    "github.com/spf13/cobra"

    "github.com/ijcd/sesh/internal/config"
    "github.com/ijcd/sesh/internal/engine"
    "github.com/ijcd/sesh/internal/spec"
)

func newUpCmd(e *engine.Engine) *cobra.Command {
    var force bool
    var launchFlag bool
    cmd := &cobra.Command{
        Use:   "up <name>",
        Short: "Launch a project",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            p, err := config.Load(args[0], e.Drivers(), nil)
            if err != nil {
                return err
            }
            if err := requiresKitty(p, launchFlag); err != nil {
                return err
            }
            ctx := context.Background()
            if needsLaunch(p, launchFlag) {
                if err := performLaunch(ctx, p); err != nil {
                    return err
                }
            }
            if err := e.Up(ctx, p, force); err != nil {
                return err
            }
            if p.Attach == nil || *p.Attach {
                return attachIfTmux(p)
            }
            return nil
        },
    }
    cmd.Flags().BoolVar(&force, "force", false, "Down + Up if a session already exists")
    cmd.Flags().BoolVar(&launchFlag, "launch", false, "Spawn a new kitty if not already inside one (kitty driver only)")
    return cmd
}

func attachIfTmux(p *spec.Project) error {
    if p.Driver != "tmux" {
        return nil
    }
    return attachToTmux(p)
}
```

(The old `attachToTmux` lives in `cmd/sesh/attach.go` from v0.1's T34. Keep that file unchanged.)

- [ ] **Step 5: Run all tests**

```bash
go test ./...
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/sesh/up.go cmd/sesh/launch_helper.go cmd/sesh/launch_helper_test.go
git commit -m "cli: sesh up --launch flag (kitty driver)"
```

---

## Task 19: cmd/sesh/local.go --launch flag

**Files:**
- Modify: `cmd/sesh/local.go`

- [ ] **Step 1: Mirror up.go's --launch handling**

Replace `cmd/sesh/local.go`:
```go
package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"

    "github.com/ijcd/sesh/internal/config"
    "github.com/ijcd/sesh/internal/engine"
)

func newLocalCmd(e *engine.Engine) *cobra.Command {
    var force bool
    var launchFlag bool
    cmd := &cobra.Command{
        Use:   "local",
        Short: "Run ./.sesh.yml from the current directory",
        RunE: func(cmd *cobra.Command, _ []string) error {
            cwd, err := os.Getwd()
            if err != nil {
                return err
            }
            path := filepath.Join(cwd, ".sesh.yml")
            if _, err := os.Stat(path); err != nil {
                return fmt.Errorf("no .sesh.yml in %s", cwd)
            }
            p, err := config.LoadFromPath(path, e.Drivers(), nil)
            if err != nil {
                return err
            }
            if err := requiresKitty(p, launchFlag); err != nil {
                return err
            }
            ctx := context.Background()
            if needsLaunch(p, launchFlag) {
                if err := performLaunch(ctx, p); err != nil {
                    return err
                }
            }
            if err := e.Up(ctx, p, force); err != nil {
                return err
            }
            if p.Attach == nil || *p.Attach {
                return attachIfTmux(p)
            }
            return nil
        },
    }
    cmd.Flags().BoolVar(&force, "force", false, "Down + Up if a session already exists")
    cmd.Flags().BoolVar(&launchFlag, "launch", false, "Spawn a new kitty if not already inside one (kitty driver only)")
    return cmd
}
```

- [ ] **Step 2: Build and test**

```bash
go build ./cmd/sesh
go test ./...
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/sesh/local.go
git commit -m "cli: sesh local --launch flag"
```

---

## Task 20: cmd/sesh/down.go honors state for --launch cleanup

**Files:**
- Modify: `cmd/sesh/down.go`

- [ ] **Step 1: Implement state-aware Down**

Replace `cmd/sesh/down.go`:
```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/spf13/cobra"

    "github.com/ijcd/sesh/internal/config"
    "github.com/ijcd/sesh/internal/engine"
    "github.com/ijcd/sesh/internal/state"
)

func newDownCmd(e *engine.Engine) *cobra.Command {
    return &cobra.Command{
        Use:   "down <name>",
        Short: "Stop a project",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            p, err := config.Load(args[0], e.Drivers(), nil)
            if err != nil {
                return err
            }
            ctx := context.Background()

            // If this project was launched via --launch, point KITTY_LISTEN_ON
            // at its tracked socket so Down can talk to that kitty.
            statePath, err := state.DefaultPath()
            if err == nil {
                if s, err2 := state.Load(statePath); err2 == nil {
                    if entry, ok := s.Get(p.Name); ok {
                        os.Setenv("KITTY_LISTEN_ON", "unix:"+entry.Socket)
                        defer cleanupLaunch(s, p.Name, statePath, entry.Socket)
                    }
                }
            }

            return e.Down(ctx, p)
        },
    }
}

func cleanupLaunch(s *state.Store, name, statePath, socket string) {
    s.Delete(name)
    _ = s.Save(statePath)
    _ = os.Remove(socket)
}
```

- [ ] **Step 2: Build and test**

```bash
go build ./cmd/sesh
go test ./...
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/sesh/down.go
git commit -m "cli: sesh down honors state.json for --launch teardown"
```

---

## Task 21: kitty integration test (real kitten ls)

**Files:**
- Create: `internal/drivers/kitty/integration_test.go`

- [ ] **Step 1: Write integration test gated by build tag**

`internal/drivers/kitty/integration_test.go`:
```go
//go:build integration_kitty

package kitty

import (
    "context"
    "os"
    "os/exec"
    "testing"
)

func TestIntegration_KittenAvailable(t *testing.T) {
    if _, err := exec.LookPath("kitten"); err != nil {
        t.Skip("kitten not on PATH")
    }
}

func TestIntegration_LsFromRunningKitty(t *testing.T) {
    sock := os.Getenv("KITTY_LISTEN_ON_TEST")
    if sock == "" {
        t.Skip("KITTY_LISTEN_ON_TEST not set; skipping kitty integration")
    }
    t.Setenv("KITTY_LISTEN_ON", sock)
    d := New()
    p, err := d.Capture(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if p == nil {
        t.Skip("no OS windows in test kitty instance")
    }
    if p.Driver != "kitty" {
        t.Errorf("driver = %q, want kitty", p.Driver)
    }
}
```

- [ ] **Step 2: Run with the integration tag (will skip on most CI)**

```bash
go test -tags=integration_kitty ./internal/drivers/kitty/...
```
Expected: SKIP (KITTY_LISTEN_ON_TEST unset) — that's success.

- [ ] **Step 3: Commit**

```bash
git add internal/drivers/kitty/integration_test.go
git commit -m "kitty: integration test scaffold (gated integration_kitty)"
```

---

## Task 22: e2e smoke for kitty config

**Files:**
- Modify: `e2e/e2e_test.go`

- [ ] **Step 1: Add a kitty-flavored smoke test**

Append to `e2e/e2e_test.go`:
```go
func TestSmoke_KittyValidateAndDebug(t *testing.T) {
    if _, err := exec.LookPath("kitten"); err != nil {
        t.Skip("kitten not on PATH")
    }
    cfg := t.TempDir()
    env := map[string]string{"XDG_CONFIG_HOME": cfg}

    // sesh new <name> creates a tmux project by default; manually craft a kitty one
    projDir := filepath.Join(cfg, "sesh", "projects")
    if err := os.MkdirAll(projDir, 0o755); err != nil {
        t.Fatal(err)
    }
    kittyYaml := `driver: kitty
cwd: /tmp
tabs:
  - title: shell
  - title: dev
    cmd: echo dev
`
    if err := os.WriteFile(filepath.Join(projDir, "ktest.yml"), []byte(kittyYaml), 0o644); err != nil {
        t.Fatal(err)
    }

    if out, err := runSesh(t, env, "validate", "ktest"); err != nil {
        t.Fatalf("validate: %v\n%s", err, out)
    }
    if out, err := runSesh(t, env, "debug", "ktest"); err != nil {
        t.Fatalf("debug: %v\n%s", err, out)
    }
}
```

- [ ] **Step 2: Build binary and run**

```bash
go build -o sesh ./cmd/sesh
go test -tags=e2e ./e2e/...
```
Expected: PASS or SKIP (if no kitten).

- [ ] **Step 3: Commit**

```bash
git add e2e/e2e_test.go
git commit -m "e2e: smoke test for kitty driver (validate + debug)"
```

---

## Task 23: README + CLAUDE.md update for v0.2

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update README**

In README.md, update the Status section:
```markdown
## Status

**Pre-v1.** v0.2: tmux + kitty drivers, all three containment pairs (kitty/leaf, kitty/tmux, kitty/kitty). `--launch` flag spawns a fresh kitty when not already inside one. Plugin SPI for editor/browser/comms/Spaces is designed but unbuilt.
```

Update the Quick start to include a kitty example:
```markdown
## Quick start

```sh
go build -o sesh ./cmd/sesh

# Tmux example
./sesh new my-tmux-project
./sesh edit my-tmux-project        # set driver: tmux
./sesh up my-tmux-project

# Kitty example (run from inside a kitty terminal, or use --launch)
./sesh new my-kitty-project
./sesh edit my-kitty-project        # set driver: kitty
./sesh up my-kitty-project --launch
```
```

Bump roadmap:
```markdown
## Roadmap

- **v0.1**: terminal driver(s), tmux, capture, basic launch
- **v0.2** (current): kitty driver, --launch, full containment, ~160 tests
- **v0.3**: validation hardening, `down` cleanup edge cases, plugin SPI definition
- **v0.4**: first non-terminal plugin (editor — emacs)
- **v0.5**: browser plugin
- **v1.0**: comms + Spaces + templating + packaging
```

- [ ] **Step 2: Update CLAUDE.md**

Edit `CLAUDE.md`. Update the top line to:
```
v0.2 in Go (`cmd/sesh`) — tmux + kitty drivers; `bin/sesh` is the retired Python prototype kept for reference.
```

Add a new section after the existing "Architecture essentials":
```markdown
### Kitty driver specifics

- Tab title prefix `<project>:<tab>` is the project ↔ tabs association (no native session concept).
- `KITTY_LISTEN_ON` is read lazily inside Driver.Up/Down/Status/Capture (not at New() time), so cmd-layer `--launch` can set the env before engine.Up runs.
- Layout vocab is per-driver verbatim: kitty layouts (`splits`, `tall`, `fat`, `grid`, `horizontal`, `vertical`, `stack`).
- Cross-driver dispatch for kitty/tmux happens in `engine.Up` via `transformCrossDriverTabs` — the inner tmux session is created first, then the outer kitty tab launches with `cmd: tmux attach -t <inner-name>`.
- State for `--launch`'d kitty instances lives in `~/.local/state/sesh/state.json` (atomic write under flock).
```

- [ ] **Step 3: Verify**

```bash
go build ./cmd/sesh
go test ./...
```

- [ ] **Step 4: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "docs: README + CLAUDE for v0.2 (kitty driver)"
```

---

## Task 24: Final readiness check

- [ ] **Step 1: Run the full test matrix**

```bash
go test ./...
go test -tags=integration ./internal/drivers/tmux/...
go test -tags=integration_kitty ./internal/drivers/kitty/...
go build -o sesh ./cmd/sesh
go test -tags=e2e ./e2e/...
```
Expected: all green (or SKIP for kitty integration without env opt-in).

- [ ] **Step 2: Manual smoke (kitty)**

```bash
XDG_CONFIG_HOME=/tmp/sesh-readiness ./sesh new ktest
# Edit ktest.yml: change driver to kitty
XDG_CONFIG_HOME=/tmp/sesh-readiness ./sesh validate ktest
XDG_CONFIG_HOME=/tmp/sesh-readiness ./sesh debug ktest
# If running outside kitty:
XDG_CONFIG_HOME=/tmp/sesh-readiness ./sesh up ktest --launch
# Verify a new kitty window appears with the project tabs.
XDG_CONFIG_HOME=/tmp/sesh-readiness ./sesh down ktest
# Verify the kitty closed (or the project tabs at least are gone).
```

- [ ] **Step 3: Manual smoke (kitty/tmux cross-driver)**

Edit ktest.yml to a project with `driver: kitty` and a tab `driver: tmux` with two panes. Re-run validate/debug/up/down.

- [ ] **Step 4: No commit unless something needed fixing**

If the smoke surfaced a bug, fix in a focused commit. Otherwise, this task produces no commit — it's the readiness gate.

---

## Notes for the executor

- **Run `gofmt -l .` and `go vet ./...` after every task.** Any warning is a blocker.
- **Test targets:** unit (no tag) is the default. `-tags=integration` runs tmux integration. `-tags=integration_kitty` runs kitty integration (requires KITTY_LISTEN_ON_TEST env). `-tags=e2e` runs binary-level smoke.
- **The kitty driver depends on `KITTY_LISTEN_ON` lazily.** Test setup uses `t.Setenv` to control it; production reads via `os.Getenv` inside `runner()`.
- **`--launch` requires a real `kitty` binary.** Not exercised in unit tests; covered by manual smoke (T24) and the optional integration test (T21).
- **No new external Go deps.** Everything stdlib. If you find yourself reaching for a dep, stop and re-read the spec.
- **Cross-driver dispatch is the trickiest task (T15).** The transform runs BEFORE driver dispatch, mutating a copy of the project. Inner tmux sessions are named `<project>-<tab>` (slugged); the outer kitty tab becomes a leaf with `cmd: tmux attach -t <inner-name>`. Verify with the cross-driver test in T15 before moving on.
- **Pre-existing v0.1 tests must keep passing.** Run `go test ./...` after every task; treat any new failure as a regression to fix immediately.
