# sesh — project-aware workspace orchestrator

One command to bring up everything a project needs: terminal tabs, panes, hooks, and (eventually) editor / browser / comms / Spaces.

Like tmuxinator, but multi-driver from the start (tmux + kitty in v0.2; editor / browser / comms plugins planned).

## Install

```sh
go install github.com/ijcd/sesh/cmd/sesh@latest
# or build from source:
git clone https://github.com/ijcd/sesh && cd sesh && go build -o ~/.local/bin/sesh ./cmd/sesh
```

(Brew tap planned for v0.3.)

## Quick start

```sh
mkdir -p ~/.config/sesh/projects
cat > ~/.config/sesh/projects/hello.yml <<'EOF'
driver: tmux
cwd: ~
tabs:
  - title: shell
  - title: clock
    cmd: while true; do date; sleep 1; done
EOF

sesh up hello       # creates tmux session 'hello' with two tabs and attaches
sesh down hello     # tears it down
```

## Commands

| Command | What it does |
|---|---|
| `sesh up <name> [--force] [--launch]` | Launch project. `--force` recreates if running. `--launch` spawns kitty when not inside one. |
| `sesh down <name>` | Stop project, run `on_project_stop` hooks, close kitty if launched. |
| `sesh ls` | List configured projects. |
| `sesh new <name> [--from <other>]` | Create starter file. `--from` copies an existing project. |
| `sesh edit <name>` | Open project file in `$EDITOR`. |
| `sesh delete <name> [-y]` | Remove project file (prompts unless `-y`). |
| `sesh debug <name> [--commands-only]` | Print resolved spec + the exact tmux/kitten commands `up` would run. |
| `sesh capture <name>` | Snapshot current tmux/kitty state to YAML on stdout (suggest-only; never writes). |
| `sesh validate <name>` | Parse + merge + driver-validate without spawning. |
| `sesh local [--launch]` | Use `./.sesh.yml` from CWD (per-repo configs). |
| `sesh completion <shell>` | Shell completion script (cobra-built). |
| `sesh --version` | Print version. |

Project files: `~/.config/sesh/projects/<name>.yml`.
Templates (referenced via `extends:`): `~/.config/sesh/templates/<name>.yml`.
Per-repo: `./.sesh.yml` (used by `sesh local`).

## Examples

### 1. Tmux: single window with multiple panes

```yaml
# ~/.config/sesh/projects/myapp.yml
driver: tmux
cwd: ~/work/myapp
tabs:
  - title: dev
    layout: main-vertical
    panes:
      - title: server
        cmd: overmind start
      - title: db
        cmd: psql myapp_dev
      - title: logs
        cmd: tail -f log/dev.log
```

```sh
sesh up myapp
```

### 2. Kitty: tabs only (each tab is one command)

```yaml
# ~/.config/sesh/projects/myapp-kitty.yml
driver: kitty
cwd: ~/work/myapp
tabs:
  - title: claude
    cmd: claude --continue
  - title: editor
    cmd: nvim .
  - title: shell
```

```sh
sesh up myapp-kitty                 # from inside a kitty terminal
sesh up myapp-kitty --launch        # from any other terminal — spawns a fresh kitty
```

### 3. Kitty + tmux (the practical pattern)

Kitty owns the visible tabs; one tab hosts a tmux session for persistent panes survivable across SSH/restart.

```yaml
# ~/.config/sesh/projects/liberties.yml
driver: kitty
cwd: ~/work/liberties
tabs:
  - title: claude
    cmd: claude --continue
  - title: dev
    driver: tmux
    layout: main-vertical
    panes:
      - title: server
        cmd: overmind start
      - title: db
        cmd: psql liberties_dev
      - title: repl
        cmd: iex -S mix
  - title: notes
    cmd: nvim NOTES.md
```

```sh
sesh up liberties --launch
```

`sesh up` creates the tmux session detached, then opens kitty tabs; the `dev` tab is a kitty tab whose command is `tmux attach -t liberties-dev`.

### 4. Hooks: direnv, git pull, send-keys-style setup

```yaml
# ~/.config/sesh/projects/myapp.yml
driver: tmux
cwd: ~/work/myapp
hooks:
  pre:
    - direnv allow
    - git pull --ff-only
  on_project_start:
    - notify-send "myapp ready"
  on_project_stop:
    - git stash push -u -m "sesh-down wip"
pre_window:
  - source .envrc          # prepended to every pane's command
tabs:
  - title: dev
    layout: main-vertical
    panes:
      - title: server
        cmd: bin/start-server
      - title: test
        cmd: bin/watch-tests
```

`pre` runs once before tmux is invoked (failure aborts). `pre_window` is prepended to each pane's `cmd` via ` && `. `on_project_start` runs in the calling shell after the session is up. `on_project_stop` runs at `sesh down`.

### 5. Variables

```yaml
driver: tmux
cwd: ~/work/${APP}
vars:
  APP: liberties
  DB_NAME: liberties_dev
  PORT: 4000
tabs:
  - title: dev
    panes:
      - title: server
        cmd: PORT=${PORT} overmind start
      - title: db
        cmd: psql ${DB_NAME}
```

Lookup order: `vars:` > process env > error. Escape: `$${LITERAL}` renders as `${LITERAL}`.

### 6. Templates with `extends:`

For N projects of the same shape (10 Phoenix apps, 5 Rails apps), put the shape in a template and parameterize per-project.

```yaml
# ~/.config/sesh/templates/phoenix.yml
driver: kitty
hooks:
  pre: [direnv allow]
tabs:
  - title: claude
    cmd: claude --continue
  - title: dev
    driver: tmux
    layout: main-vertical
    panes:
      - title: server
        cmd: PORT=${PORT} overmind start
      - title: repl
        cmd: iex -S mix
      - title: db
        cmd: psql ${DB_NAME}
```

```yaml
# ~/.config/sesh/projects/liberties.yml
extends: phoenix
cwd: ~/work/liberties
vars:
  DB_NAME: liberties_dev
  PORT: 4000
```

```yaml
# ~/.config/sesh/projects/paramount.yml
extends: phoenix
cwd: ~/work/paramount
vars:
  DB_NAME: paramount_dev
  PORT: 4001
hooks:
  pre: [git pull]                    # appended to phoenix's [direnv allow]
tabs:
  - title: db
    drop: true                       # remove inherited db pane in dev (see merge rules)
  - title: notes
    cmd: nvim NOTES.md               # add a tab not in template
```

### 7. Per-repo: `./.sesh.yml` + `sesh local`

Drop `.sesh.yml` at a repo root for one-off configs that travel with the code:

```yaml
# ./.sesh.yml
driver: tmux
cwd: .
tabs:
  - title: dev
    cmd: make dev
  - title: test
    cmd: make watch
```

```sh
sesh local              # picks up ./.sesh.yml from CWD
sesh local --launch     # ditto, spawn kitty if needed
```

## Schema reference

| Key | Type | Notes |
|---|---|---|
| `extends` | string | Template name (resolved as `~/.config/sesh/templates/<name>.yml`) or `./relative.yml` path. Single-inheritance, transitive chain, cycle-detected. |
| `driver` | `tmux` \| `kitty` | Default `tmux`. |
| `cwd` | path | Project root. `~` and `${VAR}` expanded. |
| `session` | string | Override tmux session name (default: slugged project name). |
| `attach` | bool | Default `true`. `false` = leave session detached. |
| `startup_window` | string | Tab to focus on attach. |
| `startup_pane` | string | Pane to focus on attach. |
| `vars` | map | `${VAR}` interpolation source. |
| `hooks.pre` | string \| list | Shell commands run before driver.Up (calling shell). Aborts on non-zero. |
| `hooks.post` | string \| list | After driver.Up. |
| `hooks.on_project_start` | string \| list | After driver.Up (semantic alias of `post`). |
| `hooks.on_project_stop` | string \| list | At `sesh down`. |
| `pre_window` | string \| list | Prepended (` && `-joined) to every pane's `cmd`. |
| `tabs[].title` | string | Required. No `:` (tagging collision in kitty). |
| `tabs[].cwd` | path | Default = project cwd. Relative path joins parent cwd. |
| `tabs[].cmd` | string | Mutually exclusive with `panes`. |
| `tabs[].driver` | string | Override per tab (e.g., kitty project with one `driver: tmux` tab). |
| `tabs[].layout` | string | Driver-specific layout name. tmux: `tiled`/`main-vertical`/etc. kitty: `splits`/`tall`/etc. |
| `tabs[].pre_window` | string \| list | Tab-level pre_window, concatenated after project-level. |
| `tabs[].panes[].title` | string | Required. |
| `tabs[].panes[].cmd` | string | Required. |
| `tabs[].panes[].cwd` | path | Default = tab cwd. Relative path joins parent. |
| `drop: true` | on tab/pane | Sentinel — remove this entry from the inherited template. |

**Merge rules** (when `extends:` is used):

| Shape | Rule |
|---|---|
| Scalars | child wins |
| Hashes (maps) | deep-merge per key |
| Lists of titled items (`tabs`, `panes`) | merge by title; child-only titles append; `drop: true` removes |
| Lists of strings (hooks, `pre_window`) | append (parent first, child after) |

## Troubleshooting

- **`kitty driver requires running inside kitty (KITTY_LISTEN_ON unset)`** → run from a kitty terminal, OR pass `--launch` to spawn a fresh kitty for the project.
- **Kitty `--launch` says `kitten not found`** → install kitty (provides `kitten` CLI). On macOS without PATH, sesh checks `/Applications/kitty.app/Contents/MacOS/kitten` and `/opt/homebrew/bin/kitten`.
- **Pane content lands in the wrong split** (kitty users with `set -g pane-base-index 1`) → fixed in v0.2; sesh queries `pane-base-index` at Up time.
- **`sesh up` returns immediately, no attach happens** (tmux project) → `sesh up` calls `tmux attach` (or `switch-client` if you're already inside tmux) via `syscall.Exec` to replace its own process. If that's not happening, check that the project's `attach` is not set to `false`.
- **Validation passes but `up` fails** → run `sesh debug <name>` to see the exact commands sesh would run; usually surfaces the issue.
- **YAML parse errors** → `goccy/go-yaml` reports file + line + col; check for tab/space mixing.

## Why this exists

There's no mature open-source tool that orchestrates a multi-app project workspace in one declarative spec. Closest: tmuxinator (terminal-only), Bunch (closed-source, procedural macros, no plugin SPI). See `PLAN.md` for the full gap analysis.

## Status

**Pre-v1.** v0.2 ships tmux + kitty drivers with all three containment pairs (kitty/leaf, kitty/tmux, kitty/kitty), `--launch` for non-kitty terminals, ~216 tests, full tmuxinator feature parity for the tmux layer. Editor / browser / comms / Spaces plugins are designed but unbuilt.

## Roadmap

- **v0.1** — tmux driver, tmuxinator parity ✓
- **v0.2** — kitty driver, `--launch`, full containment ✓
- **v0.3** — plugin SPI definition, distribution (brew tap), polish
- **v0.4** — first non-terminal plugin (editor — emacs)
- **v0.5** — browser plugin
- **v1.0** — comms + Spaces + templating extensions + packaging

## License

TBD (probably MIT).
