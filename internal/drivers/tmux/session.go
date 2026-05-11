// Package tmux is the tmux driver.
package tmux

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// resolveCwd joins child with parent: absolute child is returned as-is,
// relative child is joined onto parent, empty child inherits parent.
func resolveCwd(child, parent string) string {
	if child == "" {
		return parent
	}
	if filepath.IsAbs(child) {
		return child
	}
	return filepath.Join(parent, child)
}

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

// BuildCommands returns the argv lists for all tmux invocations for p.
// Each inner slice is the arguments passed to the tmux binary (without the
// "tmux" prefix itself). Used by Up (which executes them) and DryRun (which
// renders them for display via RenderCommands).
func BuildCommands(p *spec.Project) ([][]string, error) {
	return BuildCommandsWithOpts(p, BuildOpts{})
}

// BuildCommandsWithOpts is like BuildCommands but respects index options.
func BuildCommandsWithOpts(p *spec.Project, opts BuildOpts) ([][]string, error) {
	sess := sessionName(p)

	var cmds [][]string

	if len(p.Tabs) == 0 {
		return nil, fmt.Errorf("project %q has no tabs", p.Name)
	}

	first := p.Tabs[0]
	firstCwd := resolveCwd(first.Cwd, p.Cwd)
	cmds = append(cmds, []string{"new-session", "-d", "-s", sess, "-n", first.Title, "-c", firstCwd})
	cmds = append(cmds, buildTabWithOpts(sess, first, firstCwd, true, opts)...)

	for _, tab := range p.Tabs[1:] {
		tcwd := resolveCwd(tab.Cwd, p.Cwd)
		cmds = append(cmds, []string{"new-window", "-t", sess, "-n", tab.Title, "-c", tcwd})
		cmds = append(cmds, buildTabWithOpts(sess, tab, tcwd, false, opts)...)
	}

	return cmds, nil
}

func buildTabWithOpts(sess string, tab spec.Tab, tabCwd string, isFirst bool, opts BuildOpts) [][]string {
	var cmds [][]string
	target := fmt.Sprintf("%s:%s", sess, tab.Title)

	switch {
	case len(tab.Panes) > 0:
		// First pane is the window's existing pane; populate by send-keys.
		first := tab.Panes[0]
		if first.Cmd != "" {
			cmds = append(cmds, []string{"send-keys", "-t", target, first.Cmd, "Enter"})
		}
		if first.Title != "" {
			cmds = append(cmds, []string{"select-pane", "-t",
				fmt.Sprintf("%s.%d", target, opts.PaneBaseIndex), "-T", first.Title})
		}
		for i, pane := range tab.Panes[1:] {
			pcwd := resolveCwd(pane.Cwd, tabCwd)
			cmds = append(cmds, []string{"split-window", "-t", target, "-c", pcwd})
			paneTarget := fmt.Sprintf("%s.%d", target, opts.PaneBaseIndex+i+1)
			if pane.Cmd != "" {
				cmds = append(cmds, []string{"send-keys", "-t", paneTarget, pane.Cmd, "Enter"})
			}
			if pane.Title != "" {
				cmds = append(cmds, []string{"select-pane", "-t", paneTarget, "-T", pane.Title})
			}
		}
		if tab.Layout != "" {
			cmds = append(cmds, []string{"select-layout", "-t", target, tab.Layout})
		}
	case tab.Cmd != "":
		cmds = append(cmds, []string{"send-keys", "-t", target, tab.Cmd, "Enter"})
	}
	return cmds
}

// RenderCommands converts argv lists to display strings for DryRun output.
// Each string looks like `tmux new-session -d -s 'demo'` with shell quoting
// applied to value tokens (those that don't start with '-' and aren't the
// subcommand at position 0 or the bare word "Enter").
func RenderCommands(cmds [][]string) []string {
	out := make([]string, len(cmds))
	for i, argv := range cmds {
		if len(argv) == 0 {
			out[i] = "tmux"
			continue
		}
		parts := make([]string, len(argv))
		parts[0] = argv[0] // subcommand: never quoted
		for j, a := range argv[1:] {
			parts[j+1] = renderToken(a)
		}
		out[i] = "tmux " + strings.Join(parts, " ")
	}
	return out
}

// renderToken shell-quotes a single argv token for human-readable display.
// Flags (starting with '-') and the bare keyword "Enter" are returned as-is;
// all other value tokens are single-quoted. This reproduces the format the
// old string-interpolation in BuildCommands produced.
func renderToken(s string) string {
	if strings.HasPrefix(s, "-") || s == "Enter" {
		return s
	}
	return shellQuote(s)
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
