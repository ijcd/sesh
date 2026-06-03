package kitty

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// Capture parses `kitten ls` and produces one *spec.Project per OS window.
// Returns an empty slice (nil error) if no OS windows are present.
func (d *Driver) Capture(ctx context.Context) ([]*spec.Project, error) {
	r, err := d.runner(ctx)
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

	// Enrich foreground process info with pgid in ONE subprocess call.
	// Picks the POSIX process-group leader as each tab's user-typed command,
	// not whatever kitten happens to put at foreground_processes[0].
	pids := collectForegroundPids(wins)
	lookup := d.pgidLookup
	if lookup == nil {
		lookup = systemPgidLookup
	}
	pgids := lookup(ctx, pids)

	projects := make([]*spec.Project, 0, len(wins))
	for _, win := range wins {
		p := captureOSWindow(win, pgids)
		projects = append(projects, p)
	}
	return projects, nil
}

// collectForegroundPids returns all pids across every foreground_process in
// every tab in every OS window. Deduplicated for a tighter `ps` argv.
func collectForegroundPids(wins []kittyOSWindow) []int {
	seen := map[int]bool{}
	var out []int
	for _, win := range wins {
		for _, t := range win.Tabs {
			for _, w := range t.Windows {
				for _, fp := range w.ForegroundProcesses {
					if fp.Pid == 0 || seen[fp.Pid] {
						continue
					}
					seen[fp.Pid] = true
					out = append(out, fp.Pid)
				}
			}
		}
	}
	return out
}

// captureOSWindow converts a single kitty OS window into a *spec.Project.
// Project naming: if all tabs share a common "<prefix>:" sesh tag, the prefix
// becomes the project name; otherwise falls back to "kitty-os-<id>".
//
// pgids maps pid → pgid for every foreground process in the OS window;
// used to pick the user-typed command via POSIX group-leader semantics.
func captureOSWindow(win kittyOSWindow, pgids map[int]int) *spec.Project {
	p := &spec.Project{Driver: "kitty"}
	p.Name = osWindowName(win)
	cwds := []string{}
	for _, t := range win.Tabs {
		title := stripPrefix(t.Title)
		tab := spec.Tab{Title: title}
		cmds := []string{}
		for _, w := range t.Windows {
			if w.Cwd != "" {
				cwds = append(cwds, w.Cwd)
			}
			// Prefer the shell-recorded command (set by sesh init's preexec hook)
			// over process-tree analysis. The recorded command captures the user's
			// typed input verbatim, including shell functions and aliases.
			if recorded := normalizeRecordedCmd(w.UserVars["sesh_cmd"]); recorded != "" {
				cmds = append(cmds, recorded)
				continue
			}
			cmdline := pickForegroundCmd(w.ForegroundProcesses, pgids)
			if c := normalizeCmdline(cmdline); c != "" {
				cmds = append(cmds, c)
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
	return p
}

// osWindowName returns the project name for an OS window. If all tabs share a
// common "<prefix>:" sesh tag, that prefix is the name. Otherwise "kitty-os-<id>".
func osWindowName(win kittyOSWindow) string {
	if len(win.Tabs) == 0 {
		return fmt.Sprintf("kitty-os-%d", win.ID)
	}
	prefix := tabTitlePrefix(win.Tabs[0].Title)
	if prefix == "" {
		return fmt.Sprintf("kitty-os-%d", win.ID)
	}
	for _, t := range win.Tabs[1:] {
		if tabTitlePrefix(t.Title) != prefix {
			return fmt.Sprintf("kitty-os-%d", win.ID)
		}
	}
	return prefix
}

// tabTitlePrefix returns the part before the first ':' in a tab title, or ""
// if no ':' is present.
func tabTitlePrefix(title string) string {
	i := strings.IndexByte(title, ':')
	if i < 0 {
		return ""
	}
	return title[:i]
}

// pickFocused returns the focused window from wins, falling back to the first
// element when none is focused. Returns a zero-valued kittyOSWindow when wins
// is empty; callers should usually pre-check len(wins) > 0 since a zero ID is
// not meaningful, but the function does not panic on empty input.
func pickFocused(wins []kittyOSWindow) kittyOSWindow {
	if len(wins) == 0 {
		return kittyOSWindow{}
	}
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

// normalizeRecordedCmd cleans a sesh-recorded command. Returns "" for
// shell-only inputs (the user pressed enter at a prompt). Otherwise the
// command is returned mostly verbatim.
func normalizeRecordedCmd(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	// Don't dignify shell-builtin-only invocations as "the project's command"
	switch cmd {
	case "zsh", "bash", "fish", "sh", "exit":
		return ""
	}
	return cmd
}

func lastPathSegment(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		return s[i+1:]
	}
	return s
}
