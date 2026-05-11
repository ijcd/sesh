# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

v0.3 in Go (`cmd/sesh`) — tmux + kitty drivers, global config, include:, discovery hook; `bin/sesh` is the retired Python prototype kept for reference.

## What this is

`sesh` — a project-aware workspace orchestrator. One command brings up everything a project needs (terminal tabs/panes, eventually editor/browser/comms/Spaces). Pre-v1; **only the terminal layer is implemented**. See `README.md` for status and `PLAN.md` for the full design (driver model, plugin SPI, gap analysis vs. tmuxinator/Bunch/Workspaces).

## Code shape

- **Single file**: `bin/sesh` (Python 3 stdlib only, no deps, no build, no tests yet).
- **Config**: `~/.config/sesh/projects.json` (or `$XDG_CONFIG_HOME/sesh/projects.json`).
- Splitting into `sesh/{cli,engine,drivers/,plugins/}` is planned but deferred until plugins land — don't pre-refactor.

## Commands

```sh
./bin/sesh edit                  # open/create ~/.config/sesh/projects.json in $EDITOR
./bin/sesh ls                    # list configured projects
./bin/sesh up <name>             # launch a project
./bin/sesh capture <name>        # snapshot current kitty state -> JSON draft to stdout
python3 bin/sesh --help          # subcommand help
```

There is no test suite, no linter config, no packaging. Iterate by editing `bin/sesh` and re-running.

## Architecture essentials

### Per-level driver model

Each container (project / tab / pane) declares its own `driver`. Children inherit parent's driver. A leaf (`cmd` with no children) needs no driver. The driver answers "what spawns my children?":

| Level | Driver answers | Common values |
|---|---|---|
| Project | what spawns tabs | `kitty`, `wezterm`, `tmux` |
| Tab | what spawns panes within this tab | `kitty`, `tmux`, `wezterm` |

### Containment rule (`VALID_PAIRS` in `bin/sesh`)

Inner driver must be containable by outer. Terminal emulators host any process; multiplexers host processes only, not terminal-emulator UIs. Practical patterns: `(kitty, kitty)`, `(kitty, tmux)`, `(tmux, tmux)`. Crossing emulators (kitty→wezterm, etc.) is rejected at config load.

When extending driver support, **update `VALID_PAIRS` and the `validate()` function together** — `validate()` is called from `cmd_up()` before any spawning happens, so invalid combos must fail there, not later.

### Self-multiplexing commands are leaves

`devenv up`, `overmind start`, `mprocs` etc. spawn their own panes. In the spec they are leaf `cmd` strings — do **not** model their internal panes. Use `inner: tmux` only when YOU define the panes via the `panes:` key.

### Capture is suggest-only

`sesh capture` reads kitty's `@ ls` output, normalizes captured cmdlines (e.g., `tmux -L overmind-XXX...` → `overmind start` via `normalize_cmdline()`), and prints a draft JSON entry to stdout. It never writes to the config — the user pastes. Preserve this contract when extending capture (e.g., to other plugins).

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

### Legacy driver notes

- **Hardcoded kitten path**: `KITTEN = "/Applications/kitty.app/Contents/MacOS/kitten"`. macOS-only; will need a lookup if/when Linux support matters.
- **Kitty driver requires `KITTY_LISTEN_ON`**: `sesh up` (kitty driver) and `sesh capture` only work from inside a kitty window with remote control enabled. `KittyDriver.__init__` fails fast otherwise.
- **`hold=True` in `KittyDriver.spawn_tab`** wraps the cmd in `zsh -i -c '<cmd>; exec zsh -i'` to keep the tab alive after the command exits. This is intentional for ergonomics; don't "fix" it to use `kitty --hold`.
- **Apple-shipped Python 3.9 is the floor.** No `tomllib`, no walrus-only features that break 3.9. Stick to JSON for config; TOML is deferred until 3.11+ is the floor.

## When working on this repo

- For "how is X configured?" — read the relevant section of `PLAN.md` first; it captures design rationale not in the code.
- Pre-v1 means no backwards-compatibility burden yet. Prefer changing the schema cleanly over adding migration shims.
- Future plugin SPI shape is sketched in `PLAN.md` (`Plugin` ABC with `up/down/status/validate`). Honor that interface when adding the first non-terminal driver so it stays a reference for later plugins.
