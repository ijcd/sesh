# sesh v0.2 — kitty driver, design spec

**Date**: 2026-05-09
**Status**: Design approved (pending user review of this doc); implementation plan to follow
**Builds on**: v0.1 (tmux driver, [v0.1 spec](2026-05-08-sesh-go-v0.1-design.md))

## Goal

Add a `kitty` driver to sesh, enabling `driver: kitty` projects with kitty-managed tabs. Supports all three (kitty, X) containment pairs: `kitty/leaf`, `kitty/tmux`, `kitty/kitty`. No new schema. CLI gains `--launch` flag for spawning a fresh kitty when not already inside one.

## Testing emphasis (user directive)

**Lots of tests.** Every public function gets table-driven unit coverage. Every kitty subcommand we emit gets a regression-locking string-shape test. Capture parsing has fixtures for empty / single-tab / multi-tab / multi-pane / unfocused-OS-window cases. The `--launch` flow has tests for socket-wait, state-file roundtrip, and cleanup. See [Testing](#testing) for the per-component matrix.

## Strategic decisions

| Axis | Choice | Why |
|---|---|---|
| Containment scope | All 3 pairs (kitty/leaf, kitty/tmux, kitty/kitty) | Brainstorm Q1 — full coverage from v0.2; defer was rejected |
| Layout vocab | **Per-driver verbatim**: kitty layouts (`splits` etc.) for kitty driver, tmux layouts for tmux driver | Brainstorm Q2-A; cleanest abstraction is no abstraction |
| Default kitty layout | `splits` when panes present | Closest to tmux's freeform behavior |
| Off-kitty preconditions | In kitty: use `KITTY_LISTEN_ON`. Not in kitty: error UNLESS `--launch` flag, which spawns a fresh kitty. | Brainstorm Q3 |
| Command transport | Per-command `kitten @` over remote-control socket | Mirrors tmux driver's BuildCommands shape; same `Runner` seam pattern |
| Tab tagging | Title prefix `<project>:<tab-name>` | Matches Python prototype; simplest project-to-tabs association without kitty support for native sessions |
| Driver SPI changes | None | `--launch` is handled in cmd layer (spawn kitty + set env), driver reads `KITTY_LISTEN_ON` lazily |

## Architecture

```
internal/drivers/kitty/
  driver.go              Driver impl (Name/Up/Down/Status/Capture/Validate/DryRun)
  session.go             BuildCommands + helpers (mirrors tmux/session.go)
  capture.go             Parse `kitten ls` JSON → *spec.Project
  layouts.go             Allowed kitty layouts (validation)
  runner.go              Runner interface (mirrors tmux's), execRunner, kittenPath()
  match.go              `--match` expression builders
internal/drivers/kitty/launch/
  launch.go              spawn kitty + wait for socket (only used by cmd layer, not driver itself)
internal/state/
  state.go               persist project → kitty-launch socket mapping for Down
cmd/sesh/up.go           gain --launch flag; pre-launch kitty if needed before engine.Up
cmd/sesh/local.go        gain --launch flag (same shape as up)
cmd/sesh/down.go         honor state.json — when a project was launched, also close its kitty
cmd/sesh/root.go         register kitty.New() alongside tmux.New()
```

**Engine extensions** (kept minimal):
- `Driver` interface gains `AttachCommand(p *spec.Project) (string, error)` — returns the shell command to attach a parent driver to this driver's container. Tmux: `tmux attach -t <slug>`. Kitty: returns an error (kitty has no externally-attachable session).
- `engine.Up` gets a cross-driver branch: when `tab.Driver != p.Driver` and `tab.Panes` is non-empty, the engine first dispatches to the child driver (passing a synthesized one-tab project), then has the parent driver spawn a hosting tab whose `cmd` is `childDriver.AttachCommand(...)`.

The kitty driver reads `KITTY_LISTEN_ON` lazily inside `Up`/`Down`/`Status`/`Capture` — so if cmd-layer launch sets the env before engine.Up runs, the driver picks it up.

## Driver registration

`cmd/sesh/root.go`:
```go
e := engine.New()
e.Register(tmux.New())
e.Register(kitty.New())
```

## Schema

**No additions.** All existing fields apply: `driver: kitty`, `tabs[].title`, `tabs[].cwd`, `tabs[].cmd`, `tabs[].driver` (override per tab), `tabs[].layout`, `tabs[].panes[]` with required `title`, `cmd`, optional `cwd`, project-level `cwd`, `hooks`, `pre_window`, `attach`, `startup_window`, `startup_pane`, `vars`, `extends`.

Containment rules (from `engine/containment.go`) already include the full kitty matrix:
- `(kitty, kitty)`, `(kitty, tmux)`, `(kitty, "")` — already in `ValidPairs`.

The kitty driver registers with the engine; existing `Validate(p, registeredDrivers)` accepts `driver: kitty` once registered.

## Tab title convention

`<project-name>:<tab-name>` — visible in kitty's tab bar (e.g., `liberties:dev`). Single source of truth for project ↔ tabs mapping. Capture-output strips the prefix when generating a draft for a named project.

Trade-off: tab titles are now project-prefixed in the UI. Acceptable cost; Python prototype already does this. Configurable via a future `tab_title_format` field if needed.

## BuildCommands output

Each kitten invocation is a separate command. Driver.Up runs each via `Runner`. Example for project `demo` with two tabs (claude leaf + dev with two panes):

```
kitten @ launch --type=tab --tab-title='demo:claude' --cwd='/tmp' --hold -- claude --continue
kitten @ launch --type=tab --tab-title='demo:dev' --cwd='/tmp/x'
kitten @ goto-layout --match tab_title:^demo:dev$ splits
kitten @ launch --type=window --location=hsplit --match tab_title:^demo:dev$ --window-title='server' --cwd='/tmp/x' --hold -- overmind start
kitten @ launch --type=window --location=hsplit --match tab_title:^demo:dev$ --window-title='repl' --cwd='/tmp/x' --hold -- iex -S mix
kitten @ focus-tab --match tab_title:^demo:claude$
```

Notes:
- The leading `kitten @ --to <socket>` part is added by the Runner (not BuildCommands) so the build output is socket-independent and DryRun-safe.
- `--hold` keeps the kitty pane alive after cmd exits — replaces tmux's `zsh -i -c` hack.
- First pane of a tab is the tab's existing window (created by `launch --type=tab`); subsequent panes are explicit `launch --type=window`.
- `--location=hsplit` is the fixed split direction in v0.2; subsequent `goto-layout` rearranges per the chosen layout name. Per-pane direction control deferred to v0.3+.
- `goto-layout` runs once per multi-pane tab, defaulting to `splits` when not specified.
- Final `focus-tab` honors `startup_window` (defaults to first tab).
- For `kitty/tmux` (tab driver=tmux), the tab launches as `kitten launch --type=tab --tab-title=... -- /usr/local/bin/tmux attach -t <session>` after the engine has already created the tmux session via tmux driver. Cross-driver coordination handled in engine.Up's existing flow.

### Cross-driver tab dispatch (kitty/tmux)

When project `driver: kitty` but a tab has `driver: tmux` and `panes:`, the engine:
1. Synthesizes a single-tab project for the inner tmux session (`session = <project>-<tab-title>` slugged).
2. Calls `tmuxDriver.Up(ctx, innerProject)` to create the detached tmux session.
3. Calls `tmuxDriver.AttachCommand(innerProject)` to get the attach string (e.g., `tmux attach -t demo-dev`).
4. Calls `kittyDriver.Up` with the original project, but the offending tab's `Cmd` is replaced with the attach string and `Panes` is cleared (so kitty treats it as a leaf tab).

The transformation happens inside `engine.Up` before driver dispatch. Each driver sees only same-driver concerns.

This mirrors the Python prototype's `_spawn_kitty_tab_with_tmux_panes` but generalized via `AttachCommand`. The same machinery would handle wezterm/tmux later.

## `--launch` flow

When user runs `sesh up <project> --launch` (and `KITTY_LISTEN_ON` is unset):

1. cmd/sesh/up.go sees `--launch` + driver=kitty + not-in-kitty.
2. Generates socket path: `~/.local/state/sesh/sockets/<project>.sock`.
3. Spawns `kitty --listen-on=unix:<socket> --override allow_remote_control=yes --1` with detached process group.
4. Polls socket file existence up to 5s; bails on timeout.
5. Sets `os.Setenv("KITTY_LISTEN_ON", "unix:"+socket)`.
6. Persists `{<project>: {socket: <path>, pid: <kitty-pid>, launched_at: <iso8601>}}` in `~/.local/state/sesh/state.json` (atomic write).
7. Calls engine.Up. Driver reads env, finds the socket, proceeds normally.

`sesh up <project>` (no `--launch`):
- In kitty: read `KITTY_LISTEN_ON`, proceed.
- Not in kitty: error fast: "kitty driver requires running inside kitty (no KITTY_LISTEN_ON). Run again with --launch to spawn a new kitty for this project."

`sesh up <project> --launch` when already in kitty: ignore `--launch`, proceed normally with the existing `KITTY_LISTEN_ON`. (The flag is a no-op rather than an error — friendlier for users who alias `sesh up --launch`.)

`sesh down <project>`:
- Standard Down via existing engine flow (closes the project's tabs).
- After Down: if state.json has an entry for this project, **also** close the kitty OS window via `kitten @ close-os-window --match-tab title:^<project>:.*$`, then remove the socket file and the state entry.

`--launch` is also exposed on `sesh local`. Same behavior.

## State storage

`~/.local/state/sesh/state.json` (or `$XDG_STATE_HOME/sesh/state.json` if set):

```json
{
  "version": 1,
  "projects": {
    "liberties": {
      "socket": "/Users/ijcd/.local/state/sesh/sockets/liberties.sock",
      "pid": 23451,
      "launched_at": "2026-05-09T14:30:00Z"
    }
  }
}
```

Atomic write: write to `state.json.tmp`, rename. Read tolerates absence (no entries). Lockfile (`state.lock`) for concurrent-write safety, acquired via `flock(2)`.

## Validate

Driver-level checks (in `kitty.Driver.Validate`):
1. **Layout names**: each tab's `layout` must be in kitty's set: `splits`, `tall`, `fat`, `grid`, `horizontal`, `vertical`, `stack`, or empty (default to `splits`).
2. **Pane title required** (already enforced by config validate; re-stated for explicitness).
3. **`kitten` binary findable** at validate time (so `sesh validate` catches the install-missing case before user tries to `up`).
4. **Tab title contains no `:`** (would collide with our `<project>:<tab>` parsing during capture). Error message tells the user.

Off-kitty pre-condition (`KITTY_LISTEN_ON` unset and no `--launch`) is **not** a Validate error — it's environment, not schema. Driver.Up surfaces the error.

## Capture

`kitten @ ls` returns JSON:

```json
[
  {
    "is_focused": true,
    "tabs": [
      {
        "title": "demo:claude",
        "windows": [
          {
            "is_focused": true,
            "cwd": "/tmp",
            "foreground_processes": [{"cmdline": ["claude", "--continue"]}]
          }
        ]
      }
    ]
  }
]
```

Capture flow:
1. Find focused OS window; if none, take the first.
2. For each tab:
   - Strip `<project>:` prefix from title (using the user-supplied project name from `sesh capture <name>`).
   - For each window (kitty's term for pane), normalize `foreground_processes[0].cmdline`.
3. Single window per tab → `Tab.Cmd` (no Panes).
4. Multiple windows per tab → `Tab.Driver = "kitty"`, `Tab.Panes` filled with auto-titles `p1`, `p2`, …
5. Project-level `cwd` = most-common cwd across all windows.
6. Output YAML to stdout. Never write to config (suggest-only).

`normalizeCmdline` handles known cases:
- `tmux -L overmind-XXX-<random> ...` → `overmind start`
- Plain shells (`zsh`, `bash`, `-zsh`) → empty (interpreted as "just open a shell")
- Otherwise: `shlex.join(cmdline)` equivalent.

## Validate semantics summary

| Check | Where | When |
|---|---|---|
| Driver registered | `config.Validate` (engine layer feeds list) | every Load |
| Containment pairs | `engine.CheckContainment` | Up, Validate (existing — added in v0.1 fix #2) |
| Required titles, uniqueness, mutex cmd/panes | `config.Validate` | every Load |
| Kitty-specific layout names | `kitty.Driver.Validate` | every Load (driver-level extension) |
| Tab title no `:` | `kitty.Driver.Validate` | every Load |
| `kitten` binary present | `kitty.Driver.Validate` | every Load |

To make driver-level validation run alongside config-level: extend `engine.Validate` (which today only calls config-level) to also call `driver.Validate(p)` for the project's chosen driver and any tab-driver overrides. Small extension.

## Testing

User explicitly directed: **lots of tests**. The matrix:

| Layer | File | Tests planned (rough count) | What gets covered |
|---|---|---|---|
| BuildCommands — leaf tab | session_test.go | 3-4 | Single tab, with/without cmd; --hold semantics |
| BuildCommands — single-pane tab | session_test.go | 2 | --type=tab launch; matches expected --tab-title |
| BuildCommands — multi-pane tab (kitty/kitty) | session_test.go | 5-6 | First pane implicit, subsequent --type=window with --match; layout goto-layout emitted; --hsplit; window-title set; final pane order |
| BuildCommands — kitty/tmux pair | session_test.go | 3 | Tab cmd is `tmux attach -t <session>`; tmux session ordering correct (tmux driver runs first) |
| BuildCommands — focus-tab | session_test.go | 2-3 | startup_window default = first tab; explicit startup_window matches its tab; missing startup_window value is rejected by validate (not BuildCommands) |
| BuildCommands — multiple top-level tabs | session_test.go | 2 | Order, isolation |
| BuildCommands — pre_window prepended (engine layer) | engine/up_test.go | (existing tests already cover) | n/a (kitty doesn't change engine semantics) |
| Match expression builder | match_test.go | 6-8 | Escapes `:` and `^` and `$` in titles; no regex injection from user-supplied titles |
| shellQuote (kitty arg quoting) | session_test.go | 4 | spaces, single quotes, double quotes, mixed |
| Layouts validation | layouts_test.go | 8 | Each kitty layout name accepted; unknown rejected; empty default-to-splits |
| Driver — Name | driver_test.go | 1 | `"kitty"` |
| Driver — Up runs commands | driver_test.go | 4 | Happy path; runner failure aborts; KITTY_LISTEN_ON unset → error; lazy env read (set after construction works) |
| Driver — Down by title prefix | driver_test.go | 4 | Closes only matching tabs; missing project no-op; runner failure surfaced |
| Driver — Status by title prefix | driver_test.go | 3 | Match → Exists; no match → NotExists; runner error → Unknown + err |
| Driver — DryRun returns BuildCommands | driver_test.go | 1 | Identity check |
| Driver — Validate | driver_test.go | 5 | Bad layout; tab title with `:`; missing kitten binary; valid project; valid project with --launch context |
| Capture — empty / single tab / multi-tab / multi-pane / unfocused-OS-window | capture_test.go | 8-10 | Each scenario; cmdline normalization for overmind; cmdline normalization for plain shell; project-prefix stripping; cwd inference |
| --launch socket-wait | launch/launch_test.go | 4 | Socket appears within timeout → success; never appears → error; pre-existing socket → use it; permission error |
| --launch state-file roundtrip | state/state_test.go | 6 | Empty file; multi-project file; atomic write (no corruption on partial); concurrent-write via flock; missing dir → created; corrupted JSON → graceful error |
| --launch end-to-end (cmd layer) | up_test.go (cmd/sesh) | 3-4 | --launch flag triggers spawn; without flag and no env, error message; in-kitty with flag is a no-op |
| Down with --launch state cleanup | down_test.go (cmd/sesh) | 3 | Project in state → close-os-window emitted; not in state → no-op; state file cleanup atomic |
| Cross-driver dispatch (kitty/tmux) | engine/up_test.go | 2-3 | Engine sequences tmux setup before kitty tab; correct attach cmd in kitty tab |
| Integration — real kitten ls parsing | capture_integration_test.go (build tag) | 2 | Skip if `kitten` not on PATH; against canned but real kitty output |
| Integration — real kitty Up/Down | driver_integration_test.go (build tag) | 1-2 | Spawn kitty via --launch in test; create project; verify tabs exist; Down cleans up. Heavy; gated `-tags=integration_kitty`. Skipped in CI default |

**Estimated new test count: ~80** on top of existing 79. Total target after v0.2: **~160 tests**.

## Dependencies

| Pkg | Purpose | New? |
|---|---|---|
| stdlib only | exec, json, syscall (flock), os, io, encoding/json | yes — no new external deps |

We do NOT add a kitty-protocol library. All interaction is via `kitten @` exec.

## `kitten` binary path resolution

```go
func kittenPath() (string, error) {
    if p, err := exec.LookPath("kitten"); err == nil { return p, nil }
    // Fallback: kitty-bundled kitten
    if p, err := exec.LookPath("kitty"); err == nil {
        // `kitty +kitten` works as a generic shim
        return p + " +kitten", nil  // need argv split handling
    }
    // macOS-bundled paths
    for _, p := range []string{
        "/Applications/kitty.app/Contents/MacOS/kitten",
        "/opt/homebrew/bin/kitten",
    } {
        if _, err := os.Stat(p); err == nil { return p, nil }
    }
    return "", errors.New("kitten not found in PATH or known locations")
}
```

Linux/Flatpak users may need a future config knob; out of scope for v0.2.

## Out of scope (v0.3+)

- Reading user's `kitty.conf` for layout defaults or font/color settings.
- Sharing kitty environment across project tabs (kitty's `copy_env` feature).
- Image / hyperlink / GPU features (irrelevant to driver).
- Multiple OS windows per project.
- `--launch` cleanup if sesh dies between spawn and state-write (orphan kitty processes possible).
- Wezterm driver (deferred to ≥v0.3).
- `tab_title_format` schema field (configurable tagging).
- Layout direction control (`hsplit` vs `vsplit` per pane).

## Risk register

| Risk | Likelihood | Mitigation |
|---|---|---|
| Tab title collision (two projects with same prefix in same kitty) | low | Document; could uuid-suffix later |
| User manually renames tab → status/down miss it | medium | Document; capture re-syncs |
| `kitten` not on PATH on Linux installs | medium | Multi-location lookup; clear error message |
| `--launch` spawned kitty inherits sesh's TTY | medium | Use `os/exec` with `Setpgid: true`, detach stdin/stdout/stderr to /dev/null |
| Concurrent `--launch` of same project | low | Lockfile around state.json + socket existence check |
| Kitty `--listen-on` requires user's kitty.conf to have `allow_remote_control yes` | high | Use `--override allow_remote_control=yes` when launching; document for in-kitty case (user must set in conf) |
| Race: socket appears but kitty not yet ready to accept commands | medium | Retry first `kitten @ ls` 3× with 100ms backoff |

## References

- [v0.1 design spec](2026-05-08-sesh-go-v0.1-design.md) — patterns mirrored (Driver interface, Runner seam, BuildCommands shape).
- [bin/sesh](../../../bin/sesh) — Python prototype's KittyDriver: `_run`, `spawn_tab`, `list`, hold semantics, KITTY_LISTEN_ON detection. Logic to port directly.
- [kitten remote control docs](https://sw.kovidgoyal.net/kitty/remote-control/) — `launch`, `ls`, `goto-layout`, `focus-tab`, `close-os-window`, `send-text` reference.
- [PLAN.md](../../../PLAN.md) — kitty driver was always part of the v0.x roadmap.
