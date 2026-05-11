package tmux

import (
	"context"
	"fmt"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// Capture snapshots the current attached tmux session into a draft *spec.Project.
// Returns (nil, nil) if no current session (e.g., not running inside tmux).
func (d *Driver) Capture(ctx context.Context) (*spec.Project, error) {
	sess, err := d.r.RunCapture(ctx, "display-message", "-p", "#S")
	if err != nil || strings.TrimSpace(sess) == "" {
		return nil, nil
	}
	sess = strings.TrimSpace(sess)

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
