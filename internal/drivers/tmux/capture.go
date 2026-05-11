package tmux

import (
	"context"
	"fmt"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// Capture enumerates all tmux sessions and returns a draft *spec.Project for
// each one. Returns an empty slice (nil error) if tmux is not running or has
// no sessions.
func (d *Driver) Capture(ctx context.Context) ([]*spec.Project, error) {
	out, err := d.r.RunCapture(ctx, "list-sessions", "-F", "#{session_name}")
	if err != nil {
		// tmux not running or no sessions — not an error for callers.
		return nil, nil
	}
	sessions := splitNonEmpty(out)
	if len(sessions) == 0 {
		return nil, nil
	}

	var projects []*spec.Project
	for _, sess := range sessions {
		p, err := d.captureSession(ctx, sess)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

// captureSession builds a *spec.Project for a single named tmux session.
func (d *Driver) captureSession(ctx context.Context, sess string) (*spec.Project, error) {
	out, err := d.r.RunCapture(ctx, "list-windows", "-t", sess, "-F", "#{window_name}")
	if err != nil {
		return nil, err
	}
	windows := splitNonEmpty(out)

	p := &spec.Project{
		Name: sess, Driver: "tmux",
	}
	for _, w := range windows {
		tab := spec.Tab{Title: w}
		target := sess + ":" + w
		po, err := d.r.RunCapture(ctx, "list-panes", "-t", target, "-F", "#{pane_current_command}")
		if err != nil {
			return nil, err
		}
		cmds := splitNonEmpty(po)
		switch len(cmds) {
		case 0:
			// empty window; leave tab a leaf with no cmd
		case 1:
			tab.Cmd = cmds[0]
		default:
			for i, c := range cmds {
				tab.Panes = append(tab.Panes, spec.Pane{
					Title: fmt.Sprintf("p%d", i+1), Cmd: c,
				})
			}
			tab.Driver = "tmux"
		}
		p.Tabs = append(p.Tabs, tab)
	}
	return p, nil
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
