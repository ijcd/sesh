# sesh — design plan

## Vision

A single declarative spec per project, one command to bring it all up.

```yaml
projects:
  liberties_www:
    cwd: ~/work/theliberties/liberties_www
    space: 2                      # macOS Mission Control space
    terminal:
      driver: kitty
      tabs:
        - { title: claude, cmd: 'claude --continue' }
        - { title: dev, driver: tmux, panes: [...] }
    editor:
      driver: emacs
      frame: liberties_www
      buffers: [README.md, lib/]
    browser:
      driver: chrome
      profile: work
      tabs: [http://localhost:4000, https://github.com/.../...]
    comms:
      - { plugin: slack, target: "#dev-liberties" }
    hooks:
      pre: ["direnv allow", "git pull"]
      post: ["echo 'workspace up'"]
```

Run `sesh up liberties_www` →

1. Switch to macOS Space 2 (via aerospace/AppleScript)
2. Run `pre` hooks
3. Spawn terminal layout (driver: kitty)
4. Open emacs frame, load buffers
5. Open Chrome work profile + tabs
6. Open Slack to channel
7. Run `post` hooks

## Architecture

```
┌──────────────────────────────────────────────────┐
│ sesh engine                                      │
│ - reads project def (JSON/TOML/YAML)             │
│ - validates driver containment                   │
│ - dispatches to drivers + plugins                │
│ - hooks (pre/post per step)                      │
└──────────────┬───────────────────────────────────┘
               │
   ┌───────────┼───────────┬─────────────┬──────────┐
   ▼           ▼           ▼             ▼          ▼
┌───────┐  ┌───────┐  ┌────────┐    ┌────────┐ ┌─────────┐
│terminal│ │ editor │  │browser │    │comms   │ │macOS    │
│drivers │ │plugins │  │plugins │    │plugins │ │Spaces   │
├───────┤  ├───────┤  ├────────┤    ├────────┤ │aerospace│
│kitty  │  │ emacs │  │chrome  │    │slack   │ │window-  │
│tmux   │  │ vim   │  │firefox │    │discord │ │manager  │
│wezterm│  │ vscode│  │safari  │    │notion  │ └─────────┘
│iterm2 │  │ ...   │  │arc     │    │...     │
└───────┘  └───────┘  └────────┘    └────────┘
```

## Terminal driver design (current implementation)

### Per-level driver model

Each container (project / tab / pane) declares its own `driver`. Children inherit
parent's driver if unset. The driver answers "what spawns my children?":

| Level | Driver answers | Common values |
|---|---|---|
| Project | what spawns tabs | `kitty`, `wezterm`, `tmux` |
| Tab | what spawns panes within this tab | `kitty` (splits), `tmux`, `wezterm` |
| Pane (rare) | what spawns sub-panes | `tmux` usually |

A leaf (`cmd` with no children) needs no driver.

### Containment rule

**Inner driver must be containable by outer driver**: terminal emulators host
any process including multiplexers; multiplexers host only processes, not
terminal-emulator UIs.

| Project | Tab | Valid |
|---|---|---|
| kitty | kitty (splits) | ✓ |
| kitty | tmux | ✓ user's preferred config |
| kitty | leaf cmd | ✓ |
| kitty | wezterm | ✗ |
| wezterm | wezterm | ✓ |
| wezterm | tmux | ✓ |
| wezterm | leaf cmd | ✓ |
| wezterm | kitty | ✗ |
| tmux | tmux (tmuxinator-style) | ✓ |
| tmux | leaf cmd | ✓ |
| tmux | kitty/wezterm | ✗ |

The three practical patterns: `(kitty, kitty)`, `(kitty, tmux)`, `(tmux, tmux)`.

### Self-multiplexing commands

`devenv up`, `overmind start`, `mprocs`, etc. are themselves process managers
that spawn tmux internally. They are **leaf commands** in the workspace spec —
just `cmd: 'devenv up'`. The workspace tool doesn't need to know they internally
multiplex; they handle their own panes.

Use `inner: tmux` only when YOU explicitly define the panes (the `panes:` key
on a tab).

## Plugin SPI (planned, not implemented)

Each plugin handles one surface area. Must implement:

```python
class Plugin(ABC):
    name: str         # 'emacs', 'chrome', 'slack', 'spaces', etc.

    def up(self, project_name, spec, context): ...
    def down(self, project_name, spec, context): ...
    def status(self, project_name) -> dict: ...
    def validate(self, spec) -> list[str]: ...
```

`spec` is the relevant section of the project config (`spec['editor']` for the
editor plugin, etc.). `context` carries project-wide info (cwd, name, etc.).

### Plugin ideas

- **terminal**: kitty / tmux / wezterm / iterm2 — already drivers; treat as plugins
- **editor**:
  - emacs (uses `emacsclient -e ...` or `emacs --frame=...`)
  - vim/neovim (sessions via `:mksession`)
  - vscode (`code <project> -n`)
  - jetbrains (`idea <project>`)
- **browser**:
  - chrome (`google-chrome --profile-directory=... --new-window <url1> <url2>`)
  - firefox (similar)
  - arc (URL handler `arc://`)
  - safari (AppleScript, limited)
- **comms**:
  - slack (`slack://channel?team=...&id=...`)
  - notion (URL handler)
  - discord (URL handler)
- **spaces** (macOS):
  - aerospace (`aerospace workspace <n>`)
  - yabai (`yabai -m space --focus <n>`)
  - native (AppleScript via System Events — fragile)
- **hooks**: pre/post arbitrary shell

## Data shape (full vision)

```json
{
  "projects": {
    "liberties_www": {
      "cwd": "~/work/theliberties/liberties_www",
      "space": 2,
      "terminal": {
        "driver": "kitty",
        "tabs": [
          { "title": "claude", "cmd": "claude --continue" },
          { "title": "dev", "driver": "tmux", "session": "liberties-dev",
            "layout": "main-vertical",
            "panes": [
              { "cmd": "overmind start" },
              { "cmd": "psql liberties_www_dev" }
            ] }
        ]
      },
      "editor": { "driver": "emacs", "frame": "liberties_www" },
      "browser": { "driver": "chrome", "profile": "work",
                   "tabs": ["http://localhost:4000"] },
      "comms": [{ "plugin": "slack", "target": "#dev-liberties" }],
      "hooks": { "pre": ["direnv allow"], "post": [] }
    }
  }
}
```

## CLI surface

```
sesh up <name>             # bring up the workspace
sesh down <name>           # tear it down
sesh ls                    # list configured projects
sesh ls --running          # show currently active workspaces
sesh edit                  # open config in $EDITOR
sesh capture <name>        # snapshot current state → draft project entry
sesh validate              # check config without launching
sesh plugin list           # show registered plugins
sesh plugin install <X>    # install a plugin (when packaging exists)
```

## Implementation notes

- **Language**: Python 3 stdlib. Apple-shipped 3.9 is the floor. JSON config
  side-steps the "needs PyYAML" dependency. TOML can be added once 3.11+ is
  the floor.
- **Single file currently** for the terminal-only v0.1. Splitting into
  `sesh/{cli,engine,drivers/,plugins/}` makes sense once plugins are added.
- **Plugins as Python modules** importable from a `~/.config/sesh/plugins/`
  directory + entry-points for installable plugins.
- **Config validation** at load: walk the spec, check driver containment,
  check plugin specs against their schemas.
- **Distribution**: pip-installable (`pip install sesh-orchestrator` or
  similar). Single binary via `pyinstaller` or similar for those who don't
  want Python deps.

## Templates (deferred)

YAML/JSON anchors or explicit `extends` + variable substitution. Useful when
you have many projects of the same shape (e.g., 10 Phoenix projects). Skip
for v1; add when the repetition pain becomes real.

## Capture flow

`sesh capture <name>` reads current terminal state via the driver's list API,
normalizes captured cmdlines (e.g., `tmux -L overmind-XXX...` → `overmind start`),
produces a draft project entry, prints to stdout. **Capture suggests, never
commits.** User edits and pastes into config.

Future capture targets beyond terminal: emacs buffer list, Chrome tab snapshot,
Slack channel pinning. Each plugin contributes to capture.

## Existing landscape (gap analysis)

Comprehensive research (May 2026) confirmed: **no mature OSS or commercial tool
unifies terminal + editor + browser + comms + Spaces in one declarative spec
with a plugin SPI**.

Closest existing tools:

- **tmuxinator** (13.5k★, MIT, active): YAML for tmux sessions. Terminal-only.
- **tmuxp** (4.5k★, MIT, active): Python equivalent of tmuxinator.
- **smug** (~900★, MIT, active): Go, faster.
- **Bunch.app** (closed-source freeware): Plain-text macros for opening apps,
  files, URLs, running shell. No typed primitives, no plugin SPI, fragile
  multiplexer support. Closest holistic tool but uses procedural macros.
- **Workspaces.app** (paid, $20): Files, URLs, apps, terminal cmds, hooks.
  Explicitly disclaims window layout / Spaces / browser tab control.
- **Respace** (Raycast extension): Apps + URLs + terminal cmds. No Spaces,
  no plugins, no hooks.
- **hammerspoon-workspace-launcher** (low ★): Lua, save/restore window layouts.
- **spacehammer**: Hammerspoon framework, not project-orchestrator.
- **cmux** / Tmux-Orchestrator (2024-2026): AI-agent fleet managers. Wrong
  target audience — these orchestrate parallel Claude/Cursor sessions, not
  human dev workspaces.

Two structural challenges no one has solved:
1. **macOS browsers don't expose programmatic tab-group restore**. Chrome's APIs
   are limited; Safari has none. Best you can do is open URLs in a new window.
2. **macOS Mission Control Spaces aren't first-class addressable** via public
   APIs. AppleScript or aerospace CLI are the workarounds; both fragile.

These constraints don't prevent building the tool — they're just rough edges
to acknowledge.

## Time estimate

To v1 with 3-4 plugins (terminal + editor + browser + Spaces): 3-6 months
of focused weekend work.
