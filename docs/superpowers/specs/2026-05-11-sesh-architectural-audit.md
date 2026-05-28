# sesh — Architectural Audit (2026-05-11)

**Reviewed against:** `/Users/ijcd/.claude/CLAUDE.md` (Architectural Principles + Functional Core / Imperative Shell).
**State at audit:** main branch through Plan B (commit `93784b4`). 434 tests, 78.6% coverage.

## Closeout (2026-05-25)

Closed by subsequent commits — see ## Refactor batches below for original priority order.

| # | Status | Resolution |
|---|---|---|
| C1, I1 | ✅ closed | `ede4f1e` — `BuildCommands*` returns `[][]string`; split-parsers deleted. |
| I2 | ✅ closed | `2814f2f` — shared `internal/drivers/exec` package; tmux/kitty wrap it. |
| I3 | ✅ closed | `6bf33fa` — generic `mergeByTitle[T titled]`. |
| I4 | ✅ closed | `runProject` extracted to `cmd/sesh/attach.go:41`; up.go and local.go delegate. |
| I5, I6 | ✅ closed | `4684a6e` — socket threaded through `ctx` via `drivers.WithSocketHint`; no more type-assertion mutation. |
| I7 | ✅ closed | Lazy-runner tradeoff documented in `kitty/driver.go` `New()` comment. |
| I8 | ✅ closed | `dexec.FindBin` shared helper; remaining list differences are data, not duplication. |
| I9–I15, M1–M3 | open | Not yet swept. Sequence per refactor-batches plan below. |

## Principles applied

Functional core vs imperative shell; flat case over nested if/else; canonicalize before comparing; parameterize helpers in shared components; explicit typed data over string-double-duty; ports-and-adapters; suggestions-not-auto-set; fail-at-runtime-not-deploy; DRY at the helper level; tagged-tuple branching; normalize at ingestion.

Skipped (not applicable to Go/sesh): Ash/StreamData/TypeCheck, LiveView event handlers, CSS/Tailwind, Storybook, reference-vs-local model separation, deploy-pipeline guidance.

## Findings

### Critical

**C1. Round-trip through stringly-typed shell commands.** `internal/drivers/tmux/driver.go:80-88` + `internal/drivers/kitty/driver.go:80-88`. `BuildCommands*` produces strings prefixed with the binary name; `splitTmuxCommand`/`splitKittenCommand` then re-parse them back to argv before exec. Argv is the canonical form; this code goes argv→string→argv per invocation. Any quoting bug in either direction silently corrupts arguments.
**Fix:** `BuildCommands*` returns `[][]string` natively. Add a separate `renderArgvForDebug([][]string) []string` used only by `DryRun`/`Debug` for display.

### Important

| # | Where | Principle violated | Fix |
|---|---|---|---|
| I1 | `tmux/driver.go:123-160` + `kitty/driver.go:97-133` | DRY | `splitTmuxCommand`/`splitKittenCommand` byte-identical except leading-token. Subsumed by C1. |
| I2 | `tmux/driver.go:17-47` + `kitty/runner.go:15-65` | DRY + Ports/Adapters | `Runner` + `execRunner` duplicated. Same two-method interface, same socket-prefix-argv pattern. Extract `internal/drivers/exec` with one generic `NewExecRunner(binPath string, prefixArgs func() []string)`. |
| I3 | `config/merge.go:73-159` | DRY + commands-over-data | `mergeTabs`/`mergePanes`/`mergeTab`/`mergePane` 4-way duplication of merge-by-title. Collapse via Go generics: `mergeByTitle[T titled](parent, child []T, mergeOne func(p, c T) T) []T`. |
| I4 | `cmd/sesh/up.go:27-64` + `cmd/sesh/local.go:15-55` | DRY | Two near-identical orchestration bodies. Extract `runProject(ctx, p *spec.Project, force, launchFlag bool)`. |
| I5 | `tmux/driver.go:50-68` | Ports/Adapters | `Driver.WithSocket` mutates runner via type-assertion (`if er, ok := d.r.(*execRunner); ok`). Couples to concrete type, silently no-ops on fakes. Move socket into `NewExecRunner(socket string)`. |
| I6 | `kitty/driver.go:46-65` | Field-agnostic helper coupling | `runner()` lazily re-injects env-var socket into a test runner via type-assertion. Make socket source explicit; pass through `Up(ctx, p)` or always-fresh-runner. |
| I7 | `kitty/driver.go:36` | Fail at boot | `New()` returns `&Driver{}` with nil runner; missing kitten binary surfaces only at first dispatch. Document the conscious tradeoff. |
| I8 | `kitty/runner.go:76-86` + `kitty/launch/launch.go:38-50` | DRY | App-bundle fallback paths duplicated. Extract `findKittyBinaries() (kitty, kitten string, err error)`. |
| I9 | `config/loader.go:59-68` + `config/global.go:79-88` | DRY | `configDir()` defined privately in loader but `GlobalDefaultPath` reimplements the XDG-or-home resolution. |
| I10 | `tmux/capture.go:54-68` | Use stdlib | Hand-rolled `itoa` + `paneAutoTitle`. `strconv.Itoa` exists; kitty/capture.go:68 already uses `fmt.Sprintf("p%d", i+1)`. |
| I11 | `engine/up.go:18-57` | Functional core | `transformCrossDriverTabs` reads as transform but calls `cd.Up` (side effect). Either rename `dispatchCrossDriverTabs` or split pure-plan + shell-execute. |
| I12 | `cmd/sesh/down.go:26-36` | Boundary purity | `os.Setenv("KITTY_LISTEN_ON", ...)` to thread state across module boundaries. Pass socket through Driver API. |
| I13 | `cmd/sesh/down.go:29-35` | Error surfacing | Two `err == nil` checks silently swallow state-load errors. Spawned kitty leaks if state is corrupted. |
| I14 | `config/config.go:54-72` | Suggestions-not-auto-set | `applyGlobalDefaults` only handles `Driver`/`Attach`/`Vars` but Global defines `Editor`/`StateDir`/`Launch` too. Wire them OR remove from struct. |
| I15 | `engine/up.go:89-103` | Tagged tuples | 3-arm boolean cascade in `switch ... case status == StatusExists && !force`. Build an action tag (`skip` | `down_then_up` | `up`), then `switch action`. |

### Minor

| # | Where | Fix |
|---|---|---|
| M1 | `tmux/capture.go:58-68` | Hand-rolled `itoa` (overlaps I10). |
| M2 | `cmd/sesh/up.go:67-71` | `attachIfTmux` is a one-line type check. Inline. |
| M3 | `kitty/capture.go:78-85` | `pickFocused` returns `wins[0]` without bounds check; caller guards but make explicit. |

## Refactor batches (priority order)

1. **kill round-trip stringification** (C1, I1) — `BuildCommands*` returns `[][]string`; delete split-parsers.
2. **shared driver exec runner** (I2, I5, I8) — `internal/drivers/exec` package; tmux/kitty thin wrappers; socket in runner constructor.
3. **collapse cmd up/local duplication** (I4, M2) — `runProject` helper.
4. **generic mergeByTitle** (I3) — collapse 4 funcs to 1 with generics.
5. **capture cleanup** (I10, M1, M3) — shared pane-auto-name; stdlib `Itoa`; bounds-explicit `pickFocused`.
6. **engine.Up clean-up** (I11, I15) — split transform / rename; tagged-action switch.
7. **down/kitty boundary** (I12, I13) — socket through Driver API; surface state-load errors.
8. **global config wiring** (I14) — apply missing defaults OR prune struct.
9. **configDir consolidation** (I9) — `GlobalDefaultPath` calls `configDir`.

## What sesh does WELL

- **Functional core in `spec`/`config`**: `Merge`, `ExpandVars`, `Validate`, `ResolveInclude` are pure.
- **Tagged `Drop bool`** (`spec/spec.go:49,57`) over string sentinels.
- **Pipeline order explicit** (`config/config.go:22-50`): file → ResolveInclude → applyGlobalDefaults → ExpandVars → Validate.
- **Drivers behind a real interface** (`drivers/driver.go`): Capture/DryRun/AttachCommand/Validate separate from Up/Down.
- **Embedded shell snippets via `//go:embed`** (`init/init.go:9-16`): no runtime templating, no shell-out.
- **Capture is suggest-only**: prints YAML to stdout, never writes config.
- **State persistence atomic under flock** (`state/state.go:75-106`).
- **Per-branch visited maps** in include cycle detection (`config/include.go:39`).
- **`extends:` hard-errored** at validate; no migration shim.

## References

- Principles: `/Users/ijcd/.claude/CLAUDE.md`
- Plan B branch state: `93784b4` (mutation testing + cross-driver test)
- Plan A branch state: `fd91496` (testing hardening)
- v0.3 state: `7ff7000`/`534733f`
