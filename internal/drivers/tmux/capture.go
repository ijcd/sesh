package tmux

import (
	"context"
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
					Title: paneAutoTitle(i), Cmd: c,
				})
			}
			tab.Driver = "tmux"
		}
		p.Tabs = append(p.Tabs, tab)
	}
	return p, nil
}

func paneAutoTitle(i int) string {
	return "p" + itoa(i+1)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var s []byte
	for i > 0 {
		s = append([]byte{byte('0' + i%10)}, s...)
		i /= 10
	}
	return string(s)
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
