# sesh — cross-driver integration + mutation testing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Two distinct testing layers. (1) Real-environment integration test for the kitty/tmux cross-driver dispatch (today only mocked). (2) Mutation testing as a one-shot diagnostic to find tests that pass even when implementation is broken.

**Sequencing:** Run AFTER `2026-05-10-sesh-testing-hardening.md` lands. Mutation testing is more useful once test count is higher (more places for it to find weak assertions).

**Architecture:**
- Cross-driver integration: spawns real kitty via `--launch`, asserts the cross-driver pair (kitty-hosts-tmux) produces a tmux session that the kitty tab attaches to. Gated by `-tags=integration_cross` + env var opt-in (`SESH_TEST_KITTY_LAUNCH=1`); skipped in CI by default since CI has no display.
- Mutation testing: install `gremlins` ad-hoc; run against `internal/config` and `internal/engine` (highest-leverage logic); produce a report; commit findings as test improvements OR document why the mutation is acceptable. Not a continuous gate.

**Tech Stack:**
- Cross-driver: real `kitty` + `tmux` binaries; new `internal/drivers/kitty/launch_integration_test.go`
- Mutation: `github.com/go-gremlins/gremlins` (installed via `go install`, not a project dep)

**Decisions baked in:**

| Decision | Choice | Why |
|---|---|---|
| Mutation tool | `gremlins` | Active project; modern Go support; clean output. Alternative `go-mutesting` is older + less polished. |
| Mutation scope | `internal/config`, `internal/engine` only | Highest-leverage logic. Drivers covered by snapshot tests (Plan A). |
| Mutation cadence | Ad-hoc, not CI | Slow (mutation testing is O(N tests × M mutations)); useful as a periodic audit, not a gate. |
| Cross-driver env opt-in | `SESH_TEST_KITTY_LAUNCH=1` | Mirrors existing `KITTY_LISTEN_ON_TEST` pattern. |
| Cross-driver build tag | `integration_cross` | Distinct from `integration` (tmux-only) and `integration_kitty` (kitty-only). |

**Layout:**
```
internal/drivers/kitty/launch_integration_test.go         NEW — real kitty + real tmux cross-driver test
docs/testing/mutation-results.md                          NEW — first mutation report
.gitignore                                                 EXTEND — gremlins workdir
scripts/run-mutations.sh                                   NEW — gremlins runner with sane flags
```

---

## Task 1: Cross-driver integration test

**Files:**
- Create: `internal/drivers/kitty/launch_integration_test.go`

- [ ] **Step 1: Write the integration test**

`internal/drivers/kitty/launch_integration_test.go`:
```go
//go:build integration_cross

package kitty

import (
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/ijcd/sesh/internal/drivers/kitty/launch"
    "github.com/ijcd/sesh/internal/engine"
    "github.com/ijcd/sesh/internal/spec"
)

// TestIntegration_KittyHostingTmux exercises the full cross-driver dispatch
// against real kitty + real tmux. Spawns a kitty via --launch, runs Up for a
// project where the kitty has one tab hosting a tmux session with two panes,
// asserts the tmux session exists and the kitty tab's command points at it.
//
// SKIPPED unless SESH_TEST_KITTY_LAUNCH=1 (this opens a real kitty window).
func TestIntegration_KittyHostingTmux(t *testing.T) {
    if os.Getenv("SESH_TEST_KITTY_LAUNCH") != "1" {
        t.Skip("set SESH_TEST_KITTY_LAUNCH=1 to enable (opens a real kitty window)")
    }
    if _, err := exec.LookPath("kitty"); err != nil {
        t.Skip("kitty not on PATH")
    }
    if _, err := exec.LookPath("tmux"); err != nil {
        t.Skip("tmux not on PATH")
    }

    // Use isolated tmux socket so we don't pollute the user's tmux server.
    socket := "sesh-cross-test"
    runTmux := func(args ...string) ([]byte, error) {
        return exec.Command("tmux", append([]string{"-L", socket}, args...)...).CombinedOutput()
    }
    // Cleanup any stale state.
    runTmux("kill-session", "-t", "xtest-dev")

    // Spawn kitty via the launch package.
    sockDir := t.TempDir()
    sockPath, err := launch.SocketPathFor("xtest", sockDir)
    if err != nil {
        t.Fatal(err)
    }
    pid, err := launch.SpawnKitty(sockPath)
    if err != nil {
        t.Fatal(err)
    }
    defer func() {
        if proc, _ := os.FindProcess(pid); proc != nil {
            _ = proc.Kill()
        }
    }()
    if err := launch.WaitForSocket(context.Background(), sockPath, 5*time.Second); err != nil {
        t.Fatalf("kitty did not become ready: %v", err)
    }
    t.Setenv("KITTY_LISTEN_ON", "unix:"+sockPath)

    // Build a project: kitty driver, one tab hosting tmux with two panes.
    p := &spec.Project{
        Name:   "xtest",
        Driver: "kitty",
        Cwd:    "/tmp",
        Tabs: []spec.Tab{
            {Title: "shell"},
            {Title: "dev", Driver: "tmux", Panes: []spec.Pane{
                {Title: "p1", Cmd: "true"},
                {Title: "p2", Cmd: "true"},
            }},
        },
    }

    // Construct engine with both drivers; tmux driver uses isolated socket.
    e := engine.New()
    tDrv := newTmuxDriverOnSocket(socket)
    e.Register(tDrv)
    e.Register(New())

    if err := e.Up(context.Background(), p, false); err != nil {
        t.Fatalf("Up: %v", err)
    }
    defer runTmux("kill-session", "-t", "xtest-dev")

    // Assert: tmux session "xtest-dev" exists with 2 panes.
    out, err := runTmux("list-panes", "-t", "xtest-dev", "-F", "#{pane_index}")
    if err != nil {
        t.Fatalf("list-panes failed: %v\nout: %s", err, out)
    }
    panes := strings.Fields(strings.TrimSpace(string(out)))
    if len(panes) != 2 {
        t.Errorf("expected 2 panes in xtest-dev, got %d (%v)", len(panes), panes)
    }

    // Assert: kitty's "xtest:dev" tab exists and its command references tmux attach.
    d := New()
    cmds, _ := d.DryRun(p)
    foundAttach := false
    for _, c := range cmds {
        if strings.Contains(c, "xtest:dev") && strings.Contains(c, "tmux attach") {
            foundAttach = true
        }
    }
    if !foundAttach {
        t.Errorf("expected kitty's xtest:dev tab to launch with `tmux attach`, none found")
    }
}

// newTmuxDriverOnSocket — placeholder. The current tmux driver uses the user's
// default tmux socket; for an isolated test we'd inject a Runner that prefixes
// `-L sesh-cross-test`. The cleanest path is to extend the tmux package to
// expose this. For v0.4-prelude, accept this as a known limitation: this test
// runs against the user's default tmux socket. Document it.
func newTmuxDriverOnSocket(socket string) *tmuxDriverWrapper {
    // Intentionally not isolating the tmux socket in v0.4-prelude;
    // see Step 2 below for the alternative.
    return &tmuxDriverWrapper{}
}

type tmuxDriverWrapper struct{}

// (Stub — actual import + use of tmux.Driver requires a small refactor;
//  see Step 2.)
```

Note: the wrapper is a placeholder. The real path is one of two options below.

- [ ] **Step 2: Pick one socket-isolation approach**

The test needs the tmux driver to use socket `-L sesh-cross-test`. Two approaches:

**Option A (preferred — small refactor)**: extend tmux driver with a `WithSocket(socket string)` method that prefixes `-L socket` to all tmux invocations. ~10 lines.

`internal/drivers/tmux/driver.go` — add:
```go
// WithSocket configures the driver to invoke `tmux -L <socket>` for all
// subsequent calls. Used by integration tests to isolate from the user's
// tmux server. Returns the same driver for chaining.
func (d *Driver) WithSocket(socket string) *Driver {
    if er, ok := d.r.(*execRunner); ok {
        er.socketArg = "-L " + socket   // requires adding socketArg field
    }
    return d
}
```

And in `execRunner.Run` / `RunCapture`, prefix `socketArg` to args (split on space).

Then in the integration test:
```go
import "github.com/ijcd/sesh/internal/drivers/tmux"

tDrv := tmux.New().WithSocket(socket)
e.Register(tDrv)
```

**Option B (simpler — no refactor, dirtier)**: don't isolate the socket. The test pollutes the user's default tmux. Document that contributors should run `tmux kill-server` before the test if they don't want their existing sessions touched.

**Pick A.** The refactor is small and cleaner. Replace the stub `tmuxDriverWrapper` in Step 1's test with the real driver wired via `WithSocket`. Drop the placeholder code.

- [ ] **Step 3: Implement WithSocket**

Edit `internal/drivers/tmux/runner.go` (or wherever `execRunner` lives):
```go
type execRunner struct {
    socketArg string // e.g., "-L sesh-test", or "" for default socket
}

func (r *execRunner) tmuxArgs(args []string) []string {
    if r.socketArg == "" {
        return args
    }
    parts := strings.Fields(r.socketArg)
    return append(parts, args...)
}

func (r *execRunner) Run(ctx context.Context, args ...string) error {
    cmd := exec.CommandContext(ctx, "tmux", r.tmuxArgs(args)...)
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
// Same change to RunCapture.
```

And `Driver.WithSocket(socket)` as in Step 2.

- [ ] **Step 4: Run integration test (with env opt-in)**

```bash
SESH_TEST_KITTY_LAUNCH=1 go test -tags=integration_cross ./internal/drivers/kitty/...
```
Expected: opens a real kitty window briefly; PASSes; cleans up.

Also verify the test SKIPs cleanly without the env:
```bash
go test -tags=integration_cross ./internal/drivers/kitty/...
```
Expected: SKIP message.

- [ ] **Step 5: Commit**

```bash
git add internal/drivers/kitty/launch_integration_test.go internal/drivers/tmux/
git commit -m "kitty: cross-driver integration test (real kitty + real tmux, gated)"
```

---

## Task 2: Update CI doc + .gitignore for cross-driver tag

**Files:**
- Modify: `CLAUDE.md`
- Modify: `.gitignore`

- [ ] **Step 1: Document the new test tag**

Append to `CLAUDE.md`'s "v0.3 ergonomics layer" section (or create a new "Testing tags" subsection):

```markdown
### Testing tags

- (default) — fast unit tests
- `integration` — real tmux on isolated socket
- `integration_kitty` — real kitten ls (requires KITTY_LISTEN_ON_TEST env)
- `integration_cross` — real kitty + real tmux cross-driver dispatch (requires SESH_TEST_KITTY_LAUNCH=1; opens a kitty window)
- `e2e` — built binary smoke
- `e2e_docs` — README YAML examples validate
```

- [ ] **Step 2: gitignore gremlins workdir**

Append to `.gitignore`:
```
# Mutation testing
gremlins-workdir/
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md .gitignore
git commit -m "docs: testing tags + gremlins workdir gitignore"
```

---

## Task 3: gremlins installation + runner script

**Files:**
- Create: `scripts/run-mutations.sh`

- [ ] **Step 1: Install gremlins (one-time)**

```bash
go install github.com/go-gremlins/gremlins/cmd/gremlins@latest
gremlins --version
```
Expected: prints a version. If `gremlins` not found, ensure `$GOBIN` (or `$HOME/go/bin`) is on PATH.

- [ ] **Step 2: Create runner script**

`scripts/run-mutations.sh`:
```sh
#!/usr/bin/env bash
# Run gremlins mutation testing against high-leverage packages.
# This is slow (minutes per package) and should be run ad-hoc, not in CI.
set -euo pipefail

PKGS=(
    "./internal/config/..."
    "./internal/engine/..."
)

# Output directory (gitignored).
OUTDIR="${OUTDIR:-gremlins-workdir}"
mkdir -p "$OUTDIR"

for pkg in "${PKGS[@]}"; do
    safe=$(echo "$pkg" | tr '/.' '__')
    out="$OUTDIR/report-${safe}.txt"
    echo ">>> Mutating $pkg → $out"
    gremlins unleash \
        --tags="" \
        --output="$out" \
        "$pkg" || true
done

echo "Done. Reports under $OUTDIR/"
```

```bash
chmod +x scripts/run-mutations.sh
```

- [ ] **Step 3: Smoke test**

```bash
./scripts/run-mutations.sh
ls -la gremlins-workdir/
```
Expected: produces report files. The first run will take 5-15 minutes per package — that's normal.

- [ ] **Step 4: Commit**

```bash
git add scripts/run-mutations.sh
git commit -m "scripts: gremlins mutation testing runner"
```

---

## Task 4: Run mutation testing + analyze

**Files:**
- Create: `docs/testing/mutation-results.md`

- [ ] **Step 1: Run mutations**

```bash
./scripts/run-mutations.sh
```
Wait for completion (~15-30 min total).

- [ ] **Step 2: Analyze the reports**

For each report under `gremlins-workdir/`:
1. Open the report.
2. Identify mutations marked LIVED (not killed by tests). Each LIVED mutation = test gap.
3. Categorize each:
   - **Real gap**: write a test that catches this mutation. Add to test suite.
   - **Acceptable**: the mutation produces semantically-equivalent code (e.g., `<=` vs `<` on a boundary that isn't reachable). Document in the results doc.
   - **Tooling artifact**: gremlins limitations. Skip.

- [ ] **Step 3: Write up findings**

`docs/testing/mutation-results.md`:
```markdown
# Mutation testing results — sesh

**Date:** YYYY-MM-DD
**Tool:** gremlins vX.Y.Z
**Scope:** `internal/config/...`, `internal/engine/...`

## Summary

| Package | Total mutations | Killed | Lived | Equivalent (acceptable) | Real gaps |
|---|---|---|---|---|---|
| internal/config | N | N | N | N | N |
| internal/engine | N | N | N | N | N |

## Real gaps closed

[List of test additions made in response to LIVED mutations]

## Equivalent mutations (no test needed)

[List of LIVED mutations that represent semantically-equivalent code, with one-sentence justification each]

## Recommendation

Re-run mutation testing after [next significant logic change] to catch new gaps.
```

- [ ] **Step 4: Commit findings + any test additions**

```bash
git add docs/testing/mutation-results.md
# plus any test files added in Step 2's "Real gap" category
git commit -m "testing: mutation results + tests for gaps gremlins surfaced"
```

---

## Task 5: Final readiness check

- [ ] **Step 1: Verify all suites pass**

```bash
go test ./...                                                      # unit
go test -tags=integration ./internal/drivers/tmux/...              # tmux
go test -tags=integration_kitty ./internal/drivers/kitty/...       # kitty (skips without env)
go test -tags=integration_cross ./internal/drivers/kitty/...       # cross-driver (skips without env)
go test -tags=e2e ./e2e/...                                        # e2e
go test -tags=e2e_docs ./e2e/...                                   # README validation
./scripts/coverage-gate.sh                                         # coverage floor
```

- [ ] **Step 2: Optionally re-run mutations to verify gaps closed**

```bash
./scripts/run-mutations.sh
```
Expected: lower LIVED count than before.

- [ ] **Step 3: No commit unless something needed fixing.**

---

## Notes for the executor

- Plan A (`2026-05-10-sesh-testing-hardening.md`) MUST be done before this plan. Mutation testing is more useful with broader test coverage; cross-driver integration assumes Plan A's testutil package exists.
- Cross-driver test is opt-in (env var) because it opens a real GUI window. Don't enable in CI default.
- Mutation testing is slow. Don't put `./scripts/run-mutations.sh` in CI — run it locally before tagged releases or after major logic changes.
- gremlins outputs `WORKDIR/report-*.txt`. The format includes a per-mutation trace; LIVED mutations are the actionable signal. KILLED mutations are good news (your tests caught them).
- If a mutation report shows widespread LIVED mutations in a function, prefer adding a property test (Plan A's pattern) over many point tests. Properties tend to catch boundary mutations that point tests miss.
