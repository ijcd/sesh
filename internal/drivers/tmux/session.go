// Package tmux is the tmux driver.
package tmux

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// BuildOpts controls index-related build behavior.
type BuildOpts struct {
	// PaneBaseIndex mirrors tmux's pane-base-index option (default 0).
	PaneBaseIndex int
	// BaseIndex mirrors tmux's base-index option (default 0). Plumbed for
	// future use; window addressing currently uses names, not indices.
	BaseIndex int
}

// queryIntOption runs `tmux show-options -gv <name>` via r, parses the
// result as int, and returns def on any error or empty output.
func queryIntOption(ctx context.Context, r Runner, name string, def int) int {
	out, err := r.RunCapture(ctx, "show-options", "-gv", name)
	if err != nil {
		return def
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return def
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return def
	}
	return n
}

// BuildCommands returns the exact tmux invocations the driver would run for p.
// Each element is a complete command including the leading "tmux ".
// Used by both DryRun (`sesh debug`) and Up (which executes them).
func BuildCommands(p *spec.Project) ([]string, error) {
	return BuildCommandsWithOpts(p, BuildOpts{})
}

// BuildCommandsWithOpts is like BuildCommands but respects index options.
func BuildCommandsWithOpts(p *spec.Project, opts BuildOpts) ([]string, error) {
	sess := sessionName(p)
	cwd := shellQuote(p.Cwd)

	var cmds []string

	if len(p.Tabs) == 0 {
		return nil, fmt.Errorf("project %q has no tabs", p.Name)
	}

	first := p.Tabs[0]
	cmds = append(cmds, fmt.Sprintf("tmux new-session -d -s %s -n %s -c %s",
		shellQuote(sess), shellQuote(first.Title), cwd))
	cmds = append(cmds, buildTabWithOpts(sess, first, p.Cwd, true, opts)...)

	for _, tab := range p.Tabs[1:] {
		tcwd := tab.Cwd
		if tcwd == "" {
			tcwd = p.Cwd
		}
		cmds = append(cmds, fmt.Sprintf("tmux new-window -t %s -n %s -c %s",
			shellQuote(sess), shellQuote(tab.Title), shellQuote(tcwd)))
		cmds = append(cmds, buildTabWithOpts(sess, tab, tcwd, false, opts)...)
	}

	return cmds, nil
}

func buildTab(sess string, tab spec.Tab, defaultCwd string, isFirst bool) []string {
	return buildTabWithOpts(sess, tab, defaultCwd, isFirst, BuildOpts{})
}

func buildTabWithOpts(sess string, tab spec.Tab, defaultCwd string, isFirst bool, opts BuildOpts) []string {
	var cmds []string
	target := fmt.Sprintf("%s:%s", sess, tab.Title)
	tcwd := tab.Cwd
	if tcwd == "" {
		tcwd = defaultCwd
	}

	switch {
	case len(tab.Panes) > 0:
		// First pane is the window's existing pane; populate by send-keys.
		first := tab.Panes[0]
		if first.Cmd != "" {
			cmds = append(cmds, fmt.Sprintf("tmux send-keys -t %s %s Enter",
				shellQuote(target), shellQuote(first.Cmd)))
		}
		if first.Title != "" {
			cmds = append(cmds, fmt.Sprintf("tmux select-pane -t %s.%d -T %s",
				shellQuote(target), opts.PaneBaseIndex, shellQuote(first.Title)))
		}
		for i, pane := range tab.Panes[1:] {
			pcwd := pane.Cwd
			if pcwd == "" {
				pcwd = tcwd
			}
			cmds = append(cmds, fmt.Sprintf("tmux split-window -t %s -c %s",
				shellQuote(target), shellQuote(pcwd)))
			paneTarget := fmt.Sprintf("%s.%d", target, opts.PaneBaseIndex+i+1)
			if pane.Cmd != "" {
				cmds = append(cmds, fmt.Sprintf("tmux send-keys -t %s %s Enter",
					shellQuote(paneTarget), shellQuote(pane.Cmd)))
			}
			if pane.Title != "" {
				cmds = append(cmds, fmt.Sprintf("tmux select-pane -t %s -T %s",
					shellQuote(paneTarget), shellQuote(pane.Title)))
			}
		}
		if tab.Layout != "" {
			cmds = append(cmds, fmt.Sprintf("tmux select-layout -t %s %s",
				shellQuote(target), shellQuote(tab.Layout)))
		}
	case tab.Cmd != "":
		cmds = append(cmds, fmt.Sprintf("tmux send-keys -t %s %s Enter",
			shellQuote(target), shellQuote(tab.Cmd)))
	}
	return cmds
}

func sessionName(p *spec.Project) string {
	if p.Session != "" {
		return p.Session
	}
	return Slug(p.Name)
}

// Slug converts a project name to a tmux-safe session name:
// lowercase, non-alphanumeric chars replaced with '-', trimmed of leading/trailing dashes.
func Slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "session"
	}
	return out
}

// shellQuote returns s wrapped in single quotes (POSIX-safe), escaping any
// embedded single quotes as '\”.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
