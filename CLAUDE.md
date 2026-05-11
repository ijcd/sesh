# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

v0.3 in Go (`cmd/sesh`) — tmux + kitty drivers, global config, include:, discovery hook.

## What this is

`sesh` — a project-aware workspace orchestrator. One command brings up everything a project needs (terminal tabs/panes, eventually editor/browser/comms/Spaces). Pre-v1; **only the terminal layer is implemented**. See `README.md` for current status.

## Code shape

- Entry point: `cmd/sesh/` (cobra CLI); domain logic in `internal/`.
- Drivers: `internal/drivers/{kitty,tmux}/`; engine orchestration: `internal/engine/`.
- Config loading pipeline: `internal/config/`.
- Tests: `go test ./...`; integration tags: `integration`, `integration_kitty`, `integration_cross`, `e2e`, `e2e_docs`.
- Build: `go build -o sesh ./cmd/sesh`.

## Architecture essentials

### Per-level driver model

Each container (project / tab / pane) declares its own `driver`. Children inherit parent's driver. A leaf (`cmd` with no children) needs no driver. The driver answers "what spawns my children?":

| Level | Driver answers | Common values |
|---|---|---|
| Project | what spawns tabs | `kitty`, `wezterm`, `tmux` |
| Tab | what spawns panes within this tab | `kitty`, `tmux`, `wezterm` |

### Containment rule (`internal/engine/containment.go`)

Inner driver must be containable by outer. Terminal emulators host any process; multiplexers host processes only, not terminal-emulator UIs. Practical patterns: `(kitty, kitty)`, `(kitty, tmux)`, `(tmux, tmux)`. Crossing emulators (kitty→wezterm, etc.) is rejected at config load.

When extending driver support, **update `containment.go` and the `Validate` function together** — validation runs before any spawning, so invalid combos must fail there, not later.

### Self-multiplexing commands are leaves

`devenv up`, `overmind start`, `mprocs` etc. spawn their own panes. In the spec they are leaf `cmd` strings — do **not** model their internal panes. Use `inner: tmux` only when YOU define the panes via the `panes:` key.

### Capture is suggest-only

`sesh capture` enumerates and normalizes existing terminal sessions across all drivers, returning results to stdout (never writes config). Modes:

- `Driver.Capture(ctx) ([]*spec.Project, error)` returns a slice—one `Project` per session/OS-window.
- **tmux** iterates ALL sessions via `tmux list-sessions`.
- **kitty** iterates ALL OS windows via `kitten ls`; cmdlines normalized (e.g., `tmux -L overmind-XXX...` → `overmind start`) in `internal/drivers/kitty/capture.go`.
- **CLI layer** dispatches all drivers (no tmux hardcoding); `capture` flag controls output mode (no args = list, `<name>` = single, `--all` = multi-doc).
- Contract: stdout only, user pastes/edits before saving. Preserve when extending capture to new drivers.

### Kitty driver specifics

- Tab title prefix `<project>:<tab>` is the project ↔ tabs association (no native session concept).
- `KITTY_LISTEN_ON` is read lazily inside Driver.Up/Down/Status/Capture (not at New() time), so cmd-layer `--launch` can set the env before engine.Up runs.
- Layout vocab is per-driver verbatim: kitty layouts (`splits`, `tall`, `fat`, `grid`, `horizontal`, `vertical`, `stack`).
- Cross-driver dispatch for kitty/tmux happens in `engine.Up` via `transformCrossDriverTabs` — the inner tmux session is created first, then the outer kitty tab launches with `cmd: tmux attach -t <inner-name>`.
- State for `--launch`'d kitty instances lives in `~/.local/state/sesh/state.json` (atomic write under flock).

## Non-obvious gotchas

### v0.3 ergonomics layer

- Global `~/.config/sesh/config.yml` provides scalar/map defaults; lists belong in templates.
- `include:` replaces `extends:` and accepts a list (`include: [phoenix, hooks/direnv]`); `extends:` errors at validate.
- Pipeline order in `config.LoadFromPath`: file → ResolveInclude → applyGlobalDefaults → ExpandVars → Validate.
- `sesh init <shell>` emits embedded shell snippets from `internal/init/scripts/`; never spawns.
- `sesh up` (no args) reads `$SESH_PROJECT`; explicit arg always wins.

### Testing tags

- (default) — fast unit tests
- `integration` — real tmux on isolated socket
- `integration_kitty` — real kitten ls (requires KITTY_LISTEN_ON_TEST env)
- `integration_cross` — real kitty + real tmux cross-driver dispatch (requires SESH_TEST_KITTY_LAUNCH=1; opens a kitty window)
- `e2e` — built binary smoke
- `e2e_docs` — README YAML examples validate

## When working on this repo

- For "how is X configured?" — read `docs/superpowers/specs/` for design rationale not in the code.
- Pre-v1 means no backwards-compatibility burden yet. Prefer changing the schema cleanly over adding migration shims.
- Future plugin SPI (v0.4) will introduce `Driver` interface extensions for non-terminal drivers (editor, browser). Design rationale lives in `docs/superpowers/specs/`.
