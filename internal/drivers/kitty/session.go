package kitty

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// BuildCommands returns the argv lists for all kitten invocations for p.
// Each inner slice is the arguments passed to the kitten binary (without the
// "kitten" prefix itself). Used by Up (which executes them) and DryRun (which
// renders them for display via RenderCommands).
func BuildCommands(p *spec.Project) ([][]string, error) {
	if len(p.Tabs) == 0 {
		return nil, fmt.Errorf("project %q has no tabs", p.Name)
	}
	var cmds [][]string
	for _, tab := range p.Tabs {
		cmds = append(cmds, buildTab(p, tab)...)
	}
	focusTab := p.StartupWindow
	if focusTab == "" {
		focusTab = p.Tabs[0].Title
	}
	cmds = append(cmds, []string{
		"focus-tab",
		"--match", MatchTabTitle(ProjectTabTitle(p.Name, focusTab)),
	})
	return cmds, nil
}

func buildTab(p *spec.Project, tab spec.Tab) [][]string {
	title := ProjectTabTitle(p.Name, tab.Title)
	tabCwd := resolveCwd(tab.Cwd, p.Cwd)
	var cmds [][]string

	// Launch the tab. When multi-pane, the first pane is the existing tab window.
	launchArgv := []string{
		"launch",
		"--type=tab",
		"--tab-title=" + title,
		"--cwd=" + tabCwd,
	}
	var firstCmd string
	if len(tab.Panes) > 0 {
		firstCmd = tab.Panes[0].Cmd
	} else {
		firstCmd = tab.Cmd
	}
	if firstCmd != "" {
		// Wrap in sh -c so compound commands (&&, pipes, redirection) and
		// quoted arguments work. Direct exec via -- argv splits on whitespace
		// and does not interpret shell syntax.
		launchArgv = append(launchArgv, "--hold", "--", "/bin/sh", "-c", firstCmd)
	}
	cmds = append(cmds, launchArgv)

	// Set the first pane's window title (only in multi-pane mode).
	if len(tab.Panes) > 0 {
		cmds = append(cmds, []string{
			"set-window-title",
			"--match", MatchTabTitle(title),
			tab.Panes[0].Title,
		})
	}

	// Subsequent panes: split via --type=window --location=hsplit.
	for _, pane := range pickAfter(tab.Panes, 1) {
		paneCwd := resolveCwd(pane.Cwd, tabCwd)
		paneArgv := []string{
			"launch",
			"--type=window",
			"--location=hsplit",
			"--match", MatchTabTitle(title),
			"--window-title=" + pane.Title,
			"--cwd=" + paneCwd,
		}
		if pane.Cmd != "" {
			paneArgv = append(paneArgv, "--hold", "--", "/bin/sh", "-c", pane.Cmd)
		}
		cmds = append(cmds, paneArgv)
	}

	// Apply layout for multi-pane tabs.
	if len(tab.Panes) > 1 {
		layout := tab.Layout
		if layout == "" {
			layout = DefaultLayout
		}
		cmds = append(cmds, []string{
			"goto-layout",
			"--match", MatchTabTitle(title),
			layout,
		})
	}

	return cmds
}

// RenderCommands converts argv lists to display strings for DryRun output.
// Each string looks like `kitten launch --type=tab --tab-title='demo:shell'`
// with shell quoting on values, reproducing the old BuildCommands format.
func RenderCommands(cmds [][]string) []string {
	out := make([]string, len(cmds))
	for i, argv := range cmds {
		if len(argv) == 0 {
			out[i] = "kitten"
			continue
		}
		parts := make([]string, len(argv))
		parts[0] = argv[0] // subcommand: never quoted
		for j, a := range argv[1:] {
			parts[j+1] = renderKittyToken(a)
		}
		out[i] = "kitten " + strings.Join(parts, " ")
	}
	return out
}

// renderKittyToken shell-quotes a kitten argv token for human-readable display.
//   - Bare '--': returned as-is.
//   - Long flags ('--flag'): returned as-is; '--flag=value' has value quoted.
//   - Short flags ('-x') and '/bin/sh': returned as-is.
//   - All other value tokens (match patterns, layouts, titles, cmds): shellQuote.
//
// Values are always shell-quoted (matching the original string-interpolation
// format), keeping the display output copy-pasteable into a shell.
func renderKittyToken(s string) string {
	if s == "--" {
		return "--"
	}
	if strings.HasPrefix(s, "--") {
		if idx := strings.IndexByte(s, '='); idx >= 0 {
			// --flag=value → --flag='value'
			return s[:idx+1] + shellQuote(s[idx+1:])
		}
		return s
	}
	if strings.HasPrefix(s, "-") {
		return s
	}
	if s == "/bin/sh" {
		return s
	}
	return shellQuote(s)
}

// pickAfter returns s[n:], or nil if n >= len(s).
func pickAfter[T any](s []T, n int) []T {
	if n >= len(s) {
		return nil
	}
	return s[n:]
}

// resolveCwd resolves a child cwd relative to parent:
// absolute child → kept as-is; relative child → joined with parent; empty → parent.
func resolveCwd(child, parent string) string {
	if child == "" {
		return parent
	}
	if filepath.IsAbs(child) {
		return child
	}
	return filepath.Join(parent, child)
}

// shellQuote wraps s in single quotes; embedded single quotes are escaped as '\".
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
