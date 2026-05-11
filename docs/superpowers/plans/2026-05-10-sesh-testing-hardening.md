# sesh — testing hardening — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lift sesh's test coverage from 65.6% to 80%+ via property tests, fuzz tests, snapshot tests, CLI command tests, README example validation, and a CI coverage gate. Close two specific gaps surfaced by coverage analysis: `cmd/sesh` at 13% and `mergePane` at 0%.

**Architecture:** Five test categories, each with shared scaffolding under `internal/testutil/`. Property tests use `pgregory.net/rapid` (modern, stdlib-friendly, automatic shrinking). Fuzz tests use Go's stdlib `testing.F`. Snapshot tests use `go-cmp` + `testdata/golden/<package>/<name>.golden` files updatable via `-update` flag. README example validation uses an extractor that walks the markdown, pulls every YAML code block under `## Examples`, and runs `config.LoadFromPath` on each.

**Tech Stack:**
- `pgregory.net/rapid` (new dep — property testing)
- `github.com/google/go-cmp/cmp` (new dep — snapshot diffs)
- stdlib `testing.F` (fuzz)
- stdlib `bufio`, `regexp` (README extractor)

**Coverage targets:**
| Package | Current | Target |
|---|---|---|
| cmd/sesh | 13.0% | ≥60% |
| internal/config | 88.2% | ≥90% |
| internal/engine | 76.2% | ≥85% |
| internal/drivers | 68.7% | ≥80% |
| internal/state | 74.9% | ≥85% |
| internal/spec | 88.9% | ≥90% |
| **Total** | **65.6%** | **≥80%** |

CI gate (T13): fail build if total drops below 75% (sets a floor below the target so we can land work without breakage).

**Decisions baked in (no further design needed):**

| Decision | Choice | Why |
|---|---|---|
| Property lib | `pgregory.net/rapid` | Stdlib-flavored API, automatic shrinking, active maintenance. Alternative `gopter` is heavier; `testing/quick` lacks shrinking. |
| Fuzz | stdlib `testing.F` | Go 1.18+ built-in; no dep needed. |
| Snapshot tooling | `go-cmp` + `.golden` files; `-update` flag regenerates | Standard Go pattern; no extra framework. |
| Snapshot location | `testdata/golden/<pkg>/<test>.golden` | Stays adjacent to source per Go convention. |
| README extractor | Inline test (no separate package) | Single test file in `e2e/` with `-tags=e2e_docs` build tag. |
| CI gate | Total coverage floor 75% (below target so work can land) | Avoids ratchet-fights during incremental work. |

**Layout:**
```
internal/testutil/                NEW package
  rapid_gen.go                      generators for spec.Project, spec.Tab, spec.Pane (rapid)
  snapshot.go                       Equal(t, got, goldenPath, *update) helper
  rapid_gen_test.go                 sanity-checks generators don't infinite-loop
internal/config/
  merge_property_test.go            NEW — rapid properties for Merge
  vars_property_test.go             NEW — rapid properties for ExpandVars
  include_property_test.go          NEW — rapid properties for ResolveInclude
  merge_test.go                     EXTEND — TestMerge_PaneFieldOverride to close mergePane gap
internal/drivers/tmux/
  fuzz_test.go                      NEW — FuzzBuildCommands, FuzzSplitTmuxCommand
  golden_test.go                    NEW — snapshot tests for BuildCommands
  testdata/golden/*.golden          NEW — snapshot fixtures
internal/drivers/kitty/
  fuzz_test.go                      NEW — FuzzBuildCommands, FuzzSplitKittenCommand
  golden_test.go                    NEW — snapshot tests for BuildCommands
  testdata/golden/*.golden          NEW
internal/engine/
  debug_golden_test.go              NEW — snapshot of full sesh debug output
  testdata/golden/*.golden          NEW
cmd/sesh/
  up_test.go                        EXTEND — cobra in-process tests for newUpCmd
  down_test.go                      NEW — newDownCmd
  ls_test.go                        NEW — newLsCmd
  edit_test.go                      NEW — newEditCmd
  new_test.go                       NEW — newNewCmd
  delete_test.go                    NEW — newDeleteCmd
  debug_test.go                     NEW — newDebugCmd
  capture_test.go                   NEW — newCaptureCmd
  local_test.go                     NEW — newLocalCmd
  validate_test.go                  NEW — newValidateCmd
  init_test.go                      NEW — newInitCmd
  root_test.go                      NEW — newRootCmd composition
e2e/
  readme_examples_test.go           NEW — extracts + validates README YAML examples (-tags=e2e)
.github/workflows/test.yml          NEW (or extend if exists) — coverage floor gate
```

---

## Task 1: testutil — rapid generators + snapshot helper

**Files:**
- Create: `internal/testutil/rapid_gen.go`
- Create: `internal/testutil/snapshot.go`
- Create: `internal/testutil/rapid_gen_test.go`

- [ ] **Step 1: Add deps**

```bash
go get pgregory.net/rapid@latest
go get github.com/google/go-cmp/cmp@latest
```

- [ ] **Step 2: Write generators**

`internal/testutil/rapid_gen.go`:
```go
// Package testutil provides shared test scaffolding: rapid generators and
// snapshot comparison helpers.
package testutil

import (
    "pgregory.net/rapid"

    "github.com/ijcd/sesh/internal/spec"
)

// SafeName generates a string suitable for tab/pane titles (no colons,
// reasonable length, ASCII).
func SafeName() *rapid.Generator[string] {
    return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,15}`)
}

// SafeCmd generates a command string that won't cause shell-quoting drama.
func SafeCmd() *rapid.Generator[string] {
    return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9 _.-]{0,40}`)
}

// SafeCwd generates an absolute-looking POSIX path.
func SafeCwd() *rapid.Generator[string] {
    return rapid.StringMatching(`/[a-z][a-z0-9_/-]{0,30}`)
}

// PaneGen generates a spec.Pane with a required title and required cmd.
func PaneGen() *rapid.Generator[spec.Pane] {
    return rapid.Custom(func(t *rapid.T) spec.Pane {
        return spec.Pane{
            Title: SafeName().Draw(t, "title"),
            Cmd:   SafeCmd().Draw(t, "cmd"),
            Cwd:   rapid.OneOf(rapid.Just(""), SafeCwd()).Draw(t, "cwd"),
        }
    })
}

// TabGen generates a spec.Tab — either leaf (cmd, no panes) or multi-pane.
func TabGen() *rapid.Generator[spec.Tab] {
    return rapid.Custom(func(t *rapid.T) spec.Tab {
        title := SafeName().Draw(t, "title")
        isLeaf := rapid.Bool().Draw(t, "isLeaf")
        if isLeaf {
            return spec.Tab{Title: title, Cmd: SafeCmd().Draw(t, "cmd")}
        }
        panes := rapid.SliceOfNDistinct(PaneGen(), 1, 4, func(p spec.Pane) string {
            return p.Title
        }).Draw(t, "panes")
        return spec.Tab{Title: title, Panes: panes}
    })
}

// ProjectGen generates a spec.Project with 1-5 distinct-titled tabs.
func ProjectGen() *rapid.Generator[spec.Project] {
    return rapid.Custom(func(t *rapid.T) spec.Project {
        return spec.Project{
            Name:   SafeName().Draw(t, "name"),
            Driver: rapid.SampledFrom([]string{"tmux", "kitty"}).Draw(t, "driver"),
            Cwd:    SafeCwd().Draw(t, "cwd"),
            Tabs: rapid.SliceOfNDistinct(TabGen(), 1, 5, func(tab spec.Tab) string {
                return tab.Title
            }).Draw(t, "tabs"),
        }
    })
}
```

- [ ] **Step 3: Write snapshot helper**

`internal/testutil/snapshot.go`:
```go
package testutil

import (
    "flag"
    "os"
    "path/filepath"
    "testing"

    "github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update golden files")

// Equal compares got against the contents of goldenPath. Pass -update to
// regenerate the golden file. Use string forms (already-rendered) — for
// structured types, pre-format with fmt.Sprintf or yaml.Marshal.
func Equal(t *testing.T, got string, goldenPath string) {
    t.Helper()
    if *update {
        if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
            t.Fatalf("mkdir: %v", err)
        }
        if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
            t.Fatalf("write golden: %v", err)
        }
        return
    }
    want, err := os.ReadFile(goldenPath)
    if err != nil {
        t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, err)
    }
    if diff := cmp.Diff(string(want), got); diff != "" {
        t.Errorf("snapshot mismatch (-want +got):\n%s\n(run with -update if intentional)", diff)
    }
}
```

- [ ] **Step 4: Sanity test for generators**

`internal/testutil/rapid_gen_test.go`:
```go
package testutil

import (
    "testing"

    "pgregory.net/rapid"
)

func TestProjectGen_NeverPanics(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        p := ProjectGen().Draw(t, "p")
        if len(p.Tabs) == 0 {
            t.Fatal("project should have at least one tab")
        }
        for _, tab := range p.Tabs {
            if tab.Title == "" {
                t.Fatal("tab title should be non-empty")
            }
            for _, pane := range tab.Panes {
                if pane.Title == "" || pane.Cmd == "" {
                    t.Fatal("pane title and cmd should be non-empty")
                }
            }
        }
    })
}
```

- [ ] **Step 5: Verify**

```bash
go test ./internal/testutil/...
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/testutil/ go.mod go.sum
git commit -m "testutil: rapid generators + snapshot helper"
```

---

## Task 2: Close the mergePane gap

**Files:**
- Modify: `internal/config/merge_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/config/merge_test.go`:
```go
func TestMerge_PaneFieldOverrideByTitle(t *testing.T) {
    parent := &spec.Project{Tabs: []spec.Tab{{Title: "dev", Panes: []spec.Pane{
        {Title: "p1", Cmd: "old-cmd", Cwd: "/parent/cwd"},
        {Title: "p2", Cmd: "keep"},
    }}}}
    child := &spec.Project{Tabs: []spec.Tab{{Title: "dev", Panes: []spec.Pane{
        {Title: "p1", Cmd: "new-cmd"}, // override cmd; cwd should inherit from parent
    }}}}
    out := Merge(parent, child)
    if len(out.Tabs[0].Panes) != 2 {
        t.Fatalf("expected 2 panes, got %d", len(out.Tabs[0].Panes))
    }
    p1 := out.Tabs[0].Panes[0]
    if p1.Cmd != "new-cmd" {
        t.Errorf("p1.Cmd = %q, want new-cmd (child override)", p1.Cmd)
    }
    if p1.Cwd != "/parent/cwd" {
        t.Errorf("p1.Cwd = %q, want /parent/cwd (inherited from parent)", p1.Cwd)
    }
    p2 := out.Tabs[0].Panes[1]
    if p2.Cmd != "keep" {
        t.Errorf("p2 should be unchanged, got %+v", p2)
    }
}
```

- [ ] **Step 2: Run, expect PASS (mergePane already implemented)**

```bash
go test ./internal/config/... -run TestMerge_PaneFieldOverride
```
Expected: PASS — this test exercises the existing-but-untested `mergePane` function.

- [ ] **Step 3: Verify coverage delta**

```bash
go test -coverprofile=/tmp/c.out ./internal/config/... && go tool cover -func=/tmp/c.out | grep mergePane
```
Expected: `mergePane` shows >0% coverage now.

- [ ] **Step 4: Commit**

```bash
git add internal/config/merge_test.go
git commit -m "config: test mergePane field-override path"
```

---

## Task 3: Property tests — Merge

**Files:**
- Create: `internal/config/merge_property_test.go`

- [ ] **Step 1: Write properties**

`internal/config/merge_property_test.go`:
```go
package config

import (
    "testing"

    "pgregory.net/rapid"

    "github.com/ijcd/sesh/internal/spec"
    "github.com/ijcd/sesh/internal/testutil"
)

// Property: Merging a project with itself produces the same project.
func TestMerge_Idempotent(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        p := testutil.ProjectGen().Draw(t, "p")
        // Set Driver explicitly so coalesce produces stable output
        if p.Driver == "" {
            p.Driver = "tmux"
        }
        out := Merge(&p, &p)
        if out.Driver != p.Driver {
            t.Errorf("Driver changed: %q → %q", p.Driver, out.Driver)
        }
        if len(out.Tabs) != len(p.Tabs) {
            t.Errorf("tab count changed: %d → %d", len(p.Tabs), len(out.Tabs))
        }
    })
}

// Property: Merging an empty parent into a child returns the child unchanged
// for scalar fields. (Hooks lists may differ if child has them — that's OK.)
func TestMerge_EmptyParentPreservesChild(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        child := testutil.ProjectGen().Draw(t, "child")
        empty := &spec.Project{}
        out := Merge(empty, &child)
        if out.Driver != child.Driver {
            t.Errorf("Driver: got %q, want %q", out.Driver, child.Driver)
        }
        if out.Cwd != child.Cwd {
            t.Errorf("Cwd: got %q, want %q", out.Cwd, child.Cwd)
        }
        if len(out.Tabs) != len(child.Tabs) {
            t.Errorf("tab count: got %d, want %d", len(out.Tabs), len(child.Tabs))
        }
    })
}

// Property: Merging child into empty parent preserves child completely.
func TestMerge_EmptyChildPreservesParent(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        parent := testutil.ProjectGen().Draw(t, "parent")
        empty := &spec.Project{}
        out := Merge(&parent, empty)
        if len(out.Tabs) != len(parent.Tabs) {
            t.Errorf("tab count: got %d, want %d", len(out.Tabs), len(parent.Tabs))
        }
    })
}

// Property: After merge, all child-titled tabs survive (not dropped, not lost).
func TestMerge_ChildTabsPreserved(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        parent := testutil.ProjectGen().Draw(t, "parent")
        child := testutil.ProjectGen().Draw(t, "child")
        out := Merge(&parent, &child)
        // Every child tab title should appear in the output.
        outTitles := map[string]bool{}
        for _, tab := range out.Tabs {
            outTitles[tab.Title] = true
        }
        for _, tab := range child.Tabs {
            if !outTitles[tab.Title] {
                t.Errorf("child tab %q lost in merge", tab.Title)
            }
        }
    })
}
```

- [ ] **Step 2: Run**

```bash
go test ./internal/config/... -run TestMerge_
```
Expected: PASS for all 4 properties (each runs ~100 cases by default).

- [ ] **Step 3: Commit**

```bash
git add internal/config/merge_property_test.go
git commit -m "config: rapid property tests for Merge (idempotence, identity, preservation)"
```

---

## Task 4: Property tests — ExpandVars

**Files:**
- Create: `internal/config/vars_property_test.go`

- [ ] **Step 1: Write properties**

`internal/config/vars_property_test.go`:
```go
package config

import (
    "strings"
    "testing"

    "pgregory.net/rapid"

    "github.com/ijcd/sesh/internal/spec"
)

// Property: ExpandVars with no ${...} in any field is identity.
func TestExpandVars_NoVarsIsIdentity(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cwd := rapid.StringMatching(`/[a-z]{1,10}`).Draw(t, "cwd")
        cmd := rapid.StringMatching(`[a-z ]{1,20}`).Draw(t, "cmd")
        p := &spec.Project{
            Cwd: cwd,
            Tabs: []spec.Tab{{Title: "x", Cmd: cmd}},
        }
        if err := ExpandVars(p, nil); err != nil {
            t.Fatal(err)
        }
        if p.Cwd != cwd {
            t.Errorf("Cwd changed: %q → %q", cwd, p.Cwd)
        }
        if p.Tabs[0].Cmd != cmd {
            t.Errorf("Cmd changed: %q → %q", cmd, p.Tabs[0].Cmd)
        }
    })
}

// Property: $${NAME} always renders as ${NAME} regardless of vars.
func TestExpandVars_EscapeIsLiteral(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        name := rapid.StringMatching(`[A-Z][A-Z0-9_]{0,8}`).Draw(t, "name")
        prefix := rapid.StringMatching(`[a-z]{1,5}`).Draw(t, "prefix")
        suffix := rapid.StringMatching(`[a-z]{1,5}`).Draw(t, "suffix")
        cmd := prefix + " $${" + name + "} " + suffix
        p := &spec.Project{
            Vars: map[string]string{name: "REPLACED"},
            Tabs: []spec.Tab{{Title: "x", Cmd: cmd}},
        }
        if err := ExpandVars(p, nil); err != nil {
            t.Fatal(err)
        }
        want := prefix + " ${" + name + "} " + suffix
        if p.Tabs[0].Cmd != want {
            t.Errorf("got %q, want %q", p.Tabs[0].Cmd, want)
        }
        if strings.Contains(p.Tabs[0].Cmd, "REPLACED") {
            t.Errorf("escape was substituted: %q", p.Tabs[0].Cmd)
        }
    })
}
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/config/... -run TestExpandVars_NoVars
go test ./internal/config/... -run TestExpandVars_Escape
git add internal/config/vars_property_test.go
git commit -m "config: rapid property tests for ExpandVars (identity, escape)"
```

---

## Task 5: Property tests — ResolveInclude

**Files:**
- Create: `internal/config/include_property_test.go`

- [ ] **Step 1: Write property**

`internal/config/include_property_test.go`:
```go
package config

import (
    "testing"

    "pgregory.net/rapid"

    "github.com/ijcd/sesh/internal/spec"
)

// Property: ResolveInclude with no includes is identity.
func TestResolveInclude_NoIncludeIsIdentity(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        driver := rapid.SampledFrom([]string{"tmux", "kitty"}).Draw(t, "driver")
        p := &spec.Project{
            Name:   "test",
            Driver: driver,
            Tabs:   []spec.Tab{{Title: "shell"}},
        }
        out, err := ResolveInclude(p, "")
        if err != nil {
            t.Fatal(err)
        }
        if out != p {
            t.Errorf("ResolveInclude with no include should return input ptr unchanged")
        }
    })
}

// Property: After ResolveInclude, the Include field is empty (absorbed).
func TestResolveInclude_ClearsInclude(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Synthesize a project that includes nothing (empty list)
        p := &spec.Project{
            Driver: "tmux",
            Tabs:   []spec.Tab{{Title: "shell"}},
        }
        out, err := ResolveInclude(p, "")
        if err != nil {
            t.Fatal(err)
        }
        if len(out.Include) != 0 {
            t.Errorf("Include should be empty post-resolve, got %v", out.Include)
        }
    })
}
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/config/... -run TestResolveInclude_
git add internal/config/include_property_test.go
git commit -m "config: rapid property tests for ResolveInclude"
```

---

## Task 6: Fuzz tests — tmux BuildCommands + splitTmuxCommand

**Files:**
- Create: `internal/drivers/tmux/fuzz_test.go`

- [ ] **Step 1: Write fuzz tests**

`internal/drivers/tmux/fuzz_test.go`:
```go
package tmux

import (
    "testing"

    "github.com/ijcd/sesh/internal/spec"
)

// FuzzBuildCommands_NeverPanics: BuildCommands must not panic on any input.
// We feed minimal seed corpus and let the fuzzer mutate.
func FuzzBuildCommands_NeverPanics(f *testing.F) {
    f.Add("demo", "tmux", "/tmp", "shell", "echo hi")
    f.Add("demo", "tmux", "/tmp", "dev", "")
    f.Add("", "", "", "", "")
    f.Add("p", "tmux", "/x", "t", string([]byte{0, 1, 2}))

    f.Fuzz(func(t *testing.T, name, driver, cwd, tabTitle, tabCmd string) {
        defer func() {
            if r := recover(); r != nil {
                t.Errorf("BuildCommands panicked: %v\ninput: name=%q driver=%q cwd=%q tab=%q cmd=%q",
                    r, name, driver, cwd, tabTitle, tabCmd)
            }
        }()
        p := &spec.Project{
            Name:   name,
            Driver: driver,
            Cwd:    cwd,
            Tabs:   []spec.Tab{{Title: tabTitle, Cmd: tabCmd}},
        }
        _, _ = BuildCommands(p)
    })
}

// FuzzSplitTmuxCommand_NeverPanics: the parser must not panic on any input.
func FuzzSplitTmuxCommand_NeverPanics(f *testing.F) {
    f.Add("tmux new-session -d -s 'demo'")
    f.Add("tmux send-keys -t 'demo:dev.0' 'echo hi' Enter")
    f.Add("")
    f.Add("not-a-tmux-command")
    f.Add("tmux 'unterminated")
    f.Add(string([]byte{0, 0xff, 0xfe}))

    f.Fuzz(func(t *testing.T, line string) {
        defer func() {
            if r := recover(); r != nil {
                t.Errorf("splitTmuxCommand panicked: %v\ninput: %q", r, line)
            }
        }()
        _, _ = splitTmuxCommand(line)
    })
}
```

- [ ] **Step 2: Run fuzz briefly**

```bash
go test -fuzz FuzzBuildCommands_NeverPanics -fuzztime 20s ./internal/drivers/tmux/...
go test -fuzz FuzzSplitTmuxCommand_NeverPanics -fuzztime 20s ./internal/drivers/tmux/...
```
Expected: no failures within 20 seconds (corpus stored under `testdata/fuzz/`).

If a crash IS found: triage. The fuzzer writes a regression file under `testdata/fuzz/<funcname>/`. Fix the panic, re-run, commit.

- [ ] **Step 3: Run as regular test (replays corpus)**

```bash
go test ./internal/drivers/tmux/... -run Fuzz
```
Expected: PASS — runs the seed corpus + any saved crashes as fast tests.

- [ ] **Step 4: Commit**

```bash
git add internal/drivers/tmux/fuzz_test.go internal/drivers/tmux/testdata/fuzz/ 2>/dev/null || git add internal/drivers/tmux/fuzz_test.go
git commit -m "tmux: fuzz tests for BuildCommands + splitTmuxCommand"
```

---

## Task 7: Fuzz tests — kitty BuildCommands + splitKittenCommand

**Files:**
- Create: `internal/drivers/kitty/fuzz_test.go`

- [ ] **Step 1: Write fuzz tests** (mirror Task 6 with kitty types)

`internal/drivers/kitty/fuzz_test.go`:
```go
package kitty

import (
    "testing"

    "github.com/ijcd/sesh/internal/spec"
)

func FuzzBuildCommands_NeverPanics(f *testing.F) {
    f.Add("demo", "/tmp", "shell", "")
    f.Add("demo", "/tmp", "claude", "claude --continue")
    f.Add("", "", "", "")
    f.Add("p", "/x", "t", string([]byte{0, 1, 2}))

    f.Fuzz(func(t *testing.T, name, cwd, tabTitle, tabCmd string) {
        defer func() {
            if r := recover(); r != nil {
                t.Errorf("BuildCommands panicked: %v", r)
            }
        }()
        p := &spec.Project{
            Name:   name,
            Driver: "kitty",
            Cwd:    cwd,
            Tabs:   []spec.Tab{{Title: tabTitle, Cmd: tabCmd}},
        }
        _, _ = BuildCommands(p)
    })
}

func FuzzSplitKittenCommand_NeverPanics(f *testing.F) {
    f.Add("kitten launch --type=tab --tab-title='demo:shell'")
    f.Add("kitten focus-tab --match tab_title:^demo$")
    f.Add("")
    f.Add("not-a-kitten-command")
    f.Add("kitten 'unterminated")

    f.Fuzz(func(t *testing.T, line string) {
        defer func() {
            if r := recover(); r != nil {
                t.Errorf("splitKittenCommand panicked: %v\ninput: %q", r, line)
            }
        }()
        _, _ = splitKittenCommand(line)
    })
}
```

- [ ] **Step 2: Run + commit**

```bash
go test -fuzz FuzzBuildCommands_NeverPanics -fuzztime 20s ./internal/drivers/kitty/...
go test -fuzz FuzzSplitKittenCommand_NeverPanics -fuzztime 20s ./internal/drivers/kitty/...
git add internal/drivers/kitty/fuzz_test.go internal/drivers/kitty/testdata/fuzz/ 2>/dev/null || git add internal/drivers/kitty/fuzz_test.go
git commit -m "kitty: fuzz tests for BuildCommands + splitKittenCommand"
```

---

## Task 8: Snapshot tests — tmux BuildCommands

**Files:**
- Create: `internal/drivers/tmux/golden_test.go`
- Create: `internal/drivers/tmux/testdata/golden/*.golden` (via -update)

- [ ] **Step 1: Write snapshot test**

`internal/drivers/tmux/golden_test.go`:
```go
package tmux

import (
    "path/filepath"
    "strings"
    "testing"

    "github.com/ijcd/sesh/internal/spec"
    "github.com/ijcd/sesh/internal/testutil"
)

type goldenCase struct {
    name string
    p    *spec.Project
}

func goldenCases() []goldenCase {
    return []goldenCase{
        {
            name: "single-leaf-tab",
            p: &spec.Project{
                Name: "demo", Driver: "tmux", Cwd: "/tmp",
                Tabs: []spec.Tab{{Title: "shell"}},
            },
        },
        {
            name: "multi-tab-with-cmds",
            p: &spec.Project{
                Name: "demo", Driver: "tmux", Cwd: "/tmp",
                Tabs: []spec.Tab{
                    {Title: "claude", Cmd: "claude --continue"},
                    {Title: "dev", Cmd: "echo dev"},
                },
            },
        },
        {
            name: "multi-pane-with-layout",
            p: &spec.Project{
                Name: "demo", Driver: "tmux", Cwd: "/tmp",
                Tabs: []spec.Tab{{Title: "dev", Layout: "main-vertical",
                    Panes: []spec.Pane{
                        {Title: "server", Cmd: "overmind start"},
                        {Title: "db", Cmd: "psql"},
                    }}},
            },
        },
    }
}

func TestBuildCommands_Golden(t *testing.T) {
    for _, tc := range goldenCases() {
        t.Run(tc.name, func(t *testing.T) {
            cmds, err := BuildCommands(tc.p)
            if err != nil {
                t.Fatal(err)
            }
            got := strings.Join(cmds, "\n") + "\n"
            golden := filepath.Join("testdata", "golden", tc.name+".golden")
            testutil.Equal(t, got, golden)
        })
    }
}
```

- [ ] **Step 2: Generate golden files**

```bash
go test ./internal/drivers/tmux/... -run TestBuildCommands_Golden -update
```
Expected: creates files under `internal/drivers/tmux/testdata/golden/`. Inspect them — verify the commands look right.

- [ ] **Step 3: Run without -update (lock the format)**

```bash
go test ./internal/drivers/tmux/... -run TestBuildCommands_Golden
```
Expected: PASS — golden files now lock the wire format.

- [ ] **Step 4: Commit**

```bash
git add internal/drivers/tmux/golden_test.go internal/drivers/tmux/testdata/golden/
git commit -m "tmux: golden snapshot tests for BuildCommands"
```

---

## Task 9: Snapshot tests — kitty BuildCommands

**Files:**
- Create: `internal/drivers/kitty/golden_test.go`
- Create: `internal/drivers/kitty/testdata/golden/*.golden`

- [ ] **Step 1: Write snapshot test** (mirror Task 8 with kitty cases)

`internal/drivers/kitty/golden_test.go`:
```go
package kitty

import (
    "path/filepath"
    "strings"
    "testing"

    "github.com/ijcd/sesh/internal/spec"
    "github.com/ijcd/sesh/internal/testutil"
)

func TestBuildCommands_Golden(t *testing.T) {
    cases := []struct {
        name string
        p    *spec.Project
    }{
        {
            name: "single-leaf-tab",
            p: &spec.Project{
                Name: "demo", Driver: "kitty", Cwd: "/tmp",
                Tabs: []spec.Tab{{Title: "shell"}},
            },
        },
        {
            name: "leaf-tab-with-cmd-shell-wrapped",
            p: &spec.Project{
                Name: "demo", Driver: "kitty", Cwd: "/tmp",
                Tabs: []spec.Tab{{Title: "claude", Cmd: "claude --continue"}},
            },
        },
        {
            name: "multi-pane-splits",
            p: &spec.Project{
                Name: "demo", Driver: "kitty", Cwd: "/tmp",
                Tabs: []spec.Tab{{Title: "dev", Driver: "kitty",
                    Panes: []spec.Pane{
                        {Title: "p1", Cmd: "x"},
                        {Title: "p2", Cmd: "y"},
                    }}},
            },
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            cmds, err := BuildCommands(tc.p)
            if err != nil {
                t.Fatal(err)
            }
            got := strings.Join(cmds, "\n") + "\n"
            golden := filepath.Join("testdata", "golden", tc.name+".golden")
            testutil.Equal(t, got, golden)
        })
    }
}
```

- [ ] **Step 2: Generate + verify + commit**

```bash
go test ./internal/drivers/kitty/... -run TestBuildCommands_Golden -update
go test ./internal/drivers/kitty/... -run TestBuildCommands_Golden
git add internal/drivers/kitty/golden_test.go internal/drivers/kitty/testdata/golden/
git commit -m "kitty: golden snapshot tests for BuildCommands"
```

---

## Task 10: Snapshot tests — engine.Debug output

**Files:**
- Create: `internal/engine/debug_golden_test.go`
- Create: `internal/engine/testdata/golden/*.golden`

- [ ] **Step 1: Write snapshot test**

`internal/engine/debug_golden_test.go`:
```go
package engine

import (
    "bytes"
    "context"
    "path/filepath"
    "testing"

    "github.com/ijcd/sesh/internal/drivers/mock"
    "github.com/ijcd/sesh/internal/spec"
    "github.com/ijcd/sesh/internal/testutil"
)

func TestDebug_Golden(t *testing.T) {
    cases := []struct {
        name string
        p    *spec.Project
    }{
        {
            name: "tmux-simple",
            p: &spec.Project{
                Name: "demo", Driver: "tmux", Cwd: "/tmp",
                Tabs: []spec.Tab{{Title: "shell"}},
            },
        },
        {
            name: "kitty-multi-tab",
            p: &spec.Project{
                Name: "demo", Driver: "kitty", Cwd: "/tmp",
                Tabs: []spec.Tab{
                    {Title: "claude", Cmd: "claude --continue"},
                    {Title: "shell"},
                },
            },
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            md := mock.New(tc.p.Driver)
            md.DryRunVal = []string{"<mocked dry-run command for " + tc.name + ">"}
            e := New()
            e.Register(md)

            var buf bytes.Buffer
            if err := e.Debug(context.Background(), tc.p, false, &buf); err != nil {
                t.Fatal(err)
            }
            golden := filepath.Join("testdata", "golden", "debug-"+tc.name+".golden")
            testutil.Equal(t, buf.String(), golden)
        })
    }
}
```

- [ ] **Step 2: Generate + verify + commit**

```bash
go test ./internal/engine/... -run TestDebug_Golden -update
go test ./internal/engine/... -run TestDebug_Golden
git add internal/engine/debug_golden_test.go internal/engine/testdata/
git commit -m "engine: golden snapshot tests for Debug output"
```

---

## Task 11: cmd/sesh — cobra in-process tests

**Files:**
- Create: `cmd/sesh/{up,down,ls,edit,new,delete,debug,capture,local,validate,init,root}_test.go` (12 files; many small)
- Modify: existing `up_test.go` and `attach_test.go` if helpful

For brevity, the plan shows ONE representative test file. The pattern repeats across all 12.

- [ ] **Step 1: Write a representative test (newLsCmd)**

`cmd/sesh/ls_test.go`:
```go
package main

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestNewLsCmd_EmptyDir(t *testing.T) {
    cfg := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", cfg)

    cmd := newLsCmd()
    var buf bytes.Buffer
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    if err := cmd.Execute(); err != nil {
        t.Fatal(err)
    }
    out := buf.String()
    if !strings.Contains(out, "no projects") {
        t.Errorf("expected empty-dir message, got %q", out)
    }
}

func TestNewLsCmd_ListsProjects(t *testing.T) {
    cfg := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", cfg)

    projDir := filepath.Join(cfg, "sesh", "projects")
    if err := os.MkdirAll(projDir, 0o755); err != nil {
        t.Fatal(err)
    }
    for _, name := range []string{"alpha.yml", "beta.yml", "gamma.yml"} {
        if err := os.WriteFile(filepath.Join(projDir, name), []byte("driver: tmux\ntabs: [{title: x}]\n"), 0o644); err != nil {
            t.Fatal(err)
        }
    }

    cmd := newLsCmd()
    var buf bytes.Buffer
    cmd.SetOut(&buf)
    if err := cmd.Execute(); err != nil {
        t.Fatal(err)
    }
    out := buf.String()
    for _, want := range []string{"alpha", "beta", "gamma"} {
        if !strings.Contains(out, want) {
            t.Errorf("output missing %q: %q", want, out)
        }
    }
}
```

Note: `newLsCmd()` currently uses `fmt.Println` directly to stdout, not the cobra cmd's Out. Adjust the implementation to use `cmd.Out()` (a 2-line change to ls.go) so tests can capture output. Apply the same fix to other commands as their tests need it.

- [ ] **Step 2: Repeat the pattern for the other 11 commands**

For each: Create a `<cmd>_test.go` file with at least 2 tests:
- Happy path
- Error path (missing project, invalid args, etc.)

Use the same fixture pattern: `t.TempDir()` for `XDG_CONFIG_HOME`, write project files, invoke the command via `cmd.Execute()`, assert on stdout.

For commands that need an Engine (up, down, debug, capture, local, validate): construct one with a mock driver. Example:
```go
e := engine.New()
md := mock.New("tmux")
e.Register(md)
cmd := newDebugCmd(e)
```

For `init`: trivial — assert output contains shell-specific markers.
For `new`: assert file is created with templated content.
For `delete`: assert file is removed (use `--yes` to skip prompt).
For `edit`: skip in tests OR mock $EDITOR with a no-op (e.g., `t.Setenv("EDITOR", "true")`).

- [ ] **Step 3: Run + verify coverage delta**

```bash
go test -cover ./cmd/sesh/...
```
Expected: coverage rises from 13% to ≥60%.

- [ ] **Step 4: Commit**

```bash
git add cmd/sesh/
git commit -m "cli: in-process cobra tests for all 12 subcommands"
```

---

## Task 12: README example validation

**Files:**
- Create: `e2e/readme_examples_test.go` (build tag `e2e_docs`)

- [ ] **Step 1: Write extractor + validator**

`e2e/readme_examples_test.go`:
```go
//go:build e2e_docs

package e2e

import (
    "bufio"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/ijcd/sesh/internal/config"
)

// TestReadmeExamples_Validate extracts every YAML code block under the
// "## Examples" section of README.md and runs it through config.LoadFromPath.
// Catches doc rot — when an example references a deprecated key or a removed
// feature, this test fails.
func TestReadmeExamples_Validate(t *testing.T) {
    f, err := os.Open(filepath.Join("..", "README.md"))
    if err != nil {
        t.Fatal(err)
    }
    defer f.Close()

    examples := extractYAMLExamples(t, f)
    if len(examples) < 5 {
        t.Fatalf("expected at least 5 README examples, got %d", len(examples))
    }

    for i, body := range examples {
        t.Run(filepath.Base(body.label), func(t *testing.T) {
            tmp := t.TempDir()
            path := filepath.Join(tmp, "example.yml")
            if err := os.WriteFile(path, []byte(body.content), 0o644); err != nil {
                t.Fatal(err)
            }
            t.Setenv("XDG_CONFIG_HOME", tmp)
            _, err := config.LoadFromPath(path, []string{"tmux", "kitty"}, nil)
            if err != nil {
                t.Errorf("example #%d (%s) failed validation: %v\n---\n%s\n---", i, body.label, err, body.content)
            }
        })
    }
}

type yamlExample struct {
    label   string
    content string
}

// extractYAMLExamples walks the markdown looking for ```yaml code fences
// inside the "## Examples" section. Returns the YAML body and the heading
// label of the example.
func extractYAMLExamples(t *testing.T, r *os.File) []yamlExample {
    t.Helper()
    var out []yamlExample
    scanner := bufio.NewScanner(r)
    inExamples := false
    inYAML := false
    var currentLabel string
    var buf strings.Builder

    for scanner.Scan() {
        line := scanner.Text()
        switch {
        case strings.HasPrefix(line, "## "):
            inExamples = strings.HasPrefix(line, "## Examples")
        case inExamples && strings.HasPrefix(line, "### "):
            currentLabel = strings.TrimPrefix(line, "### ")
        case inExamples && strings.HasPrefix(line, "```yaml"):
            inYAML = true
            buf.Reset()
        case inExamples && inYAML && strings.HasPrefix(line, "```"):
            inYAML = false
            // Skip examples that don't represent full project files
            // (e.g., template fragments, partial snippets).
            content := buf.String()
            if !looksLikeProject(content) {
                continue
            }
            out = append(out, yamlExample{label: currentLabel, content: content})
        case inExamples && inYAML:
            buf.WriteString(line)
            buf.WriteString("\n")
        }
    }
    return out
}

// looksLikeProject filters out fragment / template snippets — only validate
// content that looks like a full project (has driver: or include: + tabs).
func looksLikeProject(yaml string) bool {
    hasDriver := strings.Contains(yaml, "driver:")
    hasInclude := strings.Contains(yaml, "include:")
    hasTabs := strings.Contains(yaml, "tabs:")
    return (hasDriver || hasInclude) && hasTabs
}
```

- [ ] **Step 2: Run**

```bash
go test -tags=e2e_docs ./e2e/... -run TestReadmeExamples
```
Expected: PASS — every "full project" YAML in README validates.

If failures: either fix the README example OR adjust `looksLikeProject` to exclude that snippet (with a comment explaining why).

- [ ] **Step 3: Commit**

```bash
git add e2e/readme_examples_test.go
git commit -m "e2e: validate every README YAML example (gated e2e_docs)"
```

---

## Task 13: CI coverage gate

**Files:**
- Create: `.github/workflows/test.yml` (or extend if it exists)
- Create: `scripts/coverage-gate.sh`

- [ ] **Step 1: Create the gate script**

`scripts/coverage-gate.sh`:
```sh
#!/usr/bin/env bash
# Fail if total Go test coverage drops below the floor.
set -euo pipefail

FLOOR=${COVERAGE_FLOOR:-75}
PROFILE=${COVERAGE_PROFILE:-/tmp/sesh-cover.out}

go test -coverprofile="$PROFILE" ./... > /dev/null
TOTAL=$(go tool cover -func="$PROFILE" | tail -1 | awk '{print $3}' | sed 's/%//')

echo "Total coverage: ${TOTAL}% (floor: ${FLOOR}%)"

# Bash arithmetic doesn't do floats; use awk for comparison.
if awk "BEGIN {exit !(${TOTAL} < ${FLOOR})}"; then
    echo "FAIL: coverage ${TOTAL}% is below floor ${FLOOR}%"
    exit 1
fi
echo "OK: coverage ${TOTAL}% >= floor ${FLOOR}%"
```

```bash
chmod +x scripts/coverage-gate.sh
```

- [ ] **Step 2: Wire into CI (or skip if no CI yet)**

If `.github/workflows/test.yml` exists, append a coverage-gate step:
```yaml
- name: Coverage gate
  run: ./scripts/coverage-gate.sh
```

If no CI workflow exists, create a minimal one:
`.github/workflows/test.yml`:
```yaml
name: test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: sudo apt-get update && sudo apt-get install -y tmux
      - run: go test ./...
      - run: go test -tags=integration ./internal/drivers/tmux/...
      - run: ./scripts/coverage-gate.sh
```

- [ ] **Step 3: Smoke test the gate locally**

```bash
./scripts/coverage-gate.sh
```
Expected: prints `OK: coverage XX% >= floor 75%`. (After Tasks 1-12, total should be 80%+.)

- [ ] **Step 4: Commit**

```bash
git add scripts/coverage-gate.sh .github/workflows/test.yml
git commit -m "ci: coverage floor gate (75%)"
```

---

## Task 14: Final readiness check

- [ ] **Step 1: Full test matrix**

```bash
go test ./...
go test -tags=integration ./internal/drivers/tmux/...
go test -tags=integration_kitty ./internal/drivers/kitty/...
go test -tags=e2e ./e2e/...
go test -tags=e2e_docs ./e2e/...
go test -fuzz . -fuzztime 30s ./internal/drivers/tmux/...    # quick fuzz pass
go test -fuzz . -fuzztime 30s ./internal/drivers/kitty/...
./scripts/coverage-gate.sh
```
Expected: all green; coverage gate passes (≥75%).

- [ ] **Step 2: Verify coverage delta**

```bash
go test -coverprofile=/tmp/c.out ./... > /dev/null
go tool cover -func=/tmp/c.out | tail -1
go tool cover -func=/tmp/c.out | awk '$3 == "0.0%" {print}' | wc -l
```
Expected: total ≥ 80%; zero-coverage function count substantially reduced from 104.

- [ ] **Step 3: No commit unless something needs fixing.**

---

## Notes for the executor

- Run `gofmt -l ./...` and `go vet ./...` after each task.
- Property tests (rapid) by default run ~100 cases per `Check`. To run more for a particular property, set `RAPID_CHECKS=1000` env var.
- Fuzz tests in CI: don't enable continuous fuzzing in CI for v0.4-prelude — fuzztime should be bounded (30s in the readiness check). Long fuzzing is for ad-hoc local runs.
- Snapshot tests: when an intentional change to BuildCommands output happens, re-run with `-update` to regenerate goldens. Review the diff carefully before committing.
- README test (T12) intentionally uses a separate build tag `e2e_docs` so doc-rot failures don't block the regular `e2e` build.
- The `cmd/sesh` cobra tests in T11 may require small adjustments to use `cmd.Out()` instead of `fmt.Println` directly. That's a real improvement (testability) — make those changes.
