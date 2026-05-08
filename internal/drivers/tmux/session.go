// Package tmux is the tmux driver.
package tmux

import (
	"fmt"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// BuildCommands returns the exact tmux invocations the driver would run for p.
// Each element is a complete command including the leading "tmux ".
// Used by both DryRun (`sesh debug`) and Up (which executes them).
func BuildCommands(p *spec.Project) ([]string, error) {
	sess := sessionName(p)
	cwd := shellQuote(p.Cwd)

	var cmds []string

	if len(p.Tabs) == 0 {
		return nil, fmt.Errorf("project %q has no tabs", p.Name)
	}

	first := p.Tabs[0]
	cmds = append(cmds, fmt.Sprintf("tmux new-session -d -s %s -n %s -c %s",
		shellQuote(sess), shellQuote(first.Title), cwd))
	cmds = append(cmds, buildTab(sess, first, p.Cwd, true)...)

	for _, tab := range p.Tabs[1:] {
		tcwd := tab.Cwd
		if tcwd == "" {
			tcwd = p.Cwd
		}
		cmds = append(cmds, fmt.Sprintf("tmux new-window -t %s -n %s -c %s",
			shellQuote(sess), shellQuote(tab.Title), shellQuote(tcwd)))
		cmds = append(cmds, buildTab(sess, tab, tcwd, false)...)
	}

	return cmds, nil
}

func buildTab(sess string, tab spec.Tab, defaultCwd string, isFirst bool) []string {
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
			cmds = append(cmds, fmt.Sprintf("tmux select-pane -t %s.0 -T %s",
				shellQuote(target), shellQuote(first.Title)))
		}
		for i, pane := range tab.Panes[1:] {
			pcwd := pane.Cwd
			if pcwd == "" {
				pcwd = tcwd
			}
			cmds = append(cmds, fmt.Sprintf("tmux split-window -t %s -c %s",
				shellQuote(target), shellQuote(pcwd)))
			paneTarget := fmt.Sprintf("%s.%d", target, i+1)
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
