# sesh — project-aware workspace orchestrator

Bring up everything a project needs in one command:

- terminal tabs/panes with named commands
- editor session (specific buffers, frame name)
- browser tabs (specific profile, specific URLs)
- comms apps (Slack channel, Notion page, Discord server)
- macOS Mission Control Space
- pre/post hooks (`direnv allow`, `git pull`, anything)

Like tmuxinator, but extended beyond the terminal — typed primitives + a plugin
SPI for each surface area.

## Status

**Pre-v1.** Go reimplementation of the tmux layer in progress, targeting tmuxinator-derived feature parity. Multi-driver future (kitty/wezterm/editor/browser/comms/Spaces) preserved in the design but unbuilt. Python prototype in `bin/sesh` retained as design reference.

## Quick start

```sh
go build -o sesh ./cmd/sesh
./sesh new my-project
./sesh edit my-project
./sesh up my-project
./sesh debug my-project        # see resolved spec + tmux commands
```

Project files live at `~/.config/sesh/projects/<name>.yml`. Reusable templates live at `~/.config/sesh/templates/`. See [docs/superpowers/specs/](docs/superpowers/specs/) for the v0.1 design.

## Why this exists

There's no mature open-source tool that orchestrates a multi-app project workspace.
Confirmed by extensive research (see `PLAN.md` for the gap analysis):

| Tool | What it covers | What it misses |
|---|---|---|
| **tmuxinator** / tmuxp / smug | tmux sessions | terminal only |
| **Bunch.app** (closed-source) | Apps, files, URLs, AppleScript, keystrokes | No typed primitives, no plugin SPI, fragile multiplexer support |
| **Workspaces.app** (paid) | Apps, files, URLs, terminal cmds, hooks | No window layouts, no Spaces, no browser tabs |
| **devenv** / **direnv** | per-project shell + processes | no UI orchestration |
| **Project.el** / **Projectile** | within emacs only | within emacs only |
| **VS Code workspaces** | within VS Code only | within VS Code only |
| **aerospace** / **yabai** / **i3** | window manager only | not project-aware |
| **Hammerspoon** | scripting framework | no project domain |
| **cmux** / Tmux-Orchestrator (2024-2026) | AI-agent fleet managers | wrong target audience |

Closest existing thing is **Bunch** (https://bunchapp.co), which is closed-source
and uses procedural macros rather than typed primitives. Cannot be extended.

## Roadmap

- **v0.1** (current): tmux driver, tmuxinator parity
- **v0.2**: kitty driver
- **v0.3**: plugin SPI definition, first non-terminal plugin (editor — emacs)
- **v0.4**: browser plugin (Chrome/Arc/Safari)
- **v0.5**: Mission Control Spaces plugin (macOS)
- **v1.0**: comms plugins (Slack, Notion), templating, docs, packaging

## License

TBD (probably MIT).
