package kitty

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// Capture parses `kitten ls` and produces a draft *spec.Project for the
// focused OS window. Returns (nil, nil) if no OS windows are present.
func (d *Driver) Capture(ctx context.Context) (*spec.Project, error) {
	r, err := d.runner()
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
	win := pickFocused(wins)

	p := &spec.Project{Driver: "kitty"}
	cwds := []string{}
	for _, t := range win.Tabs {
		title := stripPrefix(t.Title)
		tab := spec.Tab{Title: title}
		cmds := []string{}
		for _, w := range t.Windows {
			if w.Cwd != "" {
				cwds = append(cwds, w.Cwd)
			}
			if len(w.ForegroundProcesses) > 0 {
				if c := normalizeCmdline(w.ForegroundProcesses[0].Cmdline); c != "" {
					cmds = append(cmds, c)
				} else {
					cmds = append(cmds, "")
				}
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
	return p, nil
}

func pickFocused(wins []kittyOSWindow) kittyOSWindow {
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

func lastPathSegment(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		return s[i+1:]
	}
	return s
}
