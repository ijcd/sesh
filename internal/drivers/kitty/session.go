package kitty

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// BuildCommands returns the exact kitten invocations the driver would run.
// Each entry begins with "kitten" (the runner adds @ --to <socket>).
func BuildCommands(p *spec.Project) ([]string, error) {
	if len(p.Tabs) == 0 {
		return nil, fmt.Errorf("project %q has no tabs", p.Name)
	}
	var cmds []string
	for _, tab := range p.Tabs {
		cmds = append(cmds, buildTab(p, tab)...)
	}
	focusTab := p.StartupWindow
	if focusTab == "" {
		focusTab = p.Tabs[0].Title
	}
	cmds = append(cmds, fmt.Sprintf(
		"kitten focus-tab --match %s",
		shellQuote(MatchTabTitle(ProjectTabTitle(p.Name, focusTab))),
	))
	return cmds, nil
}

func buildTab(p *spec.Project, tab spec.Tab) []string {
	title := ProjectTabTitle(p.Name, tab.Title)
	tabCwd := resolveCwd(tab.Cwd, p.Cwd)
	var cmds []string

	// Launch the tab. When multi-pane, the first pane is the existing tab window.
	var sb strings.Builder
	sb.WriteString("kitten launch --type=tab")
	sb.WriteString(" --tab-title=" + shellQuote(title))
	sb.WriteString(" --cwd=" + shellQuote(tabCwd))
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
		sb.WriteString(" --hold -- /bin/sh -c " + shellQuote(firstCmd))
	}
	cmds = append(cmds, sb.String())

	// Set the first pane's window title (only in multi-pane mode).
	if len(tab.Panes) > 0 {
		cmds = append(cmds, fmt.Sprintf(
			"kitten set-window-title --match %s %s",
			shellQuote(MatchTabTitle(title)),
			shellQuote(tab.Panes[0].Title),
		))
	}

	// Subsequent panes: split via --type=window --location=hsplit.
	for _, pane := range pickAfter(tab.Panes, 1) {
		paneCwd := resolveCwd(pane.Cwd, tabCwd)
		var psb strings.Builder
		psb.WriteString("kitten launch --type=window --location=hsplit")
		psb.WriteString(" --match " + shellQuote(MatchTabTitle(title)))
		psb.WriteString(" --window-title=" + shellQuote(pane.Title))
		psb.WriteString(" --cwd=" + shellQuote(paneCwd))
		if pane.Cmd != "" {
			psb.WriteString(" --hold -- /bin/sh -c " + shellQuote(pane.Cmd))
		}
		cmds = append(cmds, psb.String())
	}

	// Apply layout for multi-pane tabs.
	if len(tab.Panes) > 1 {
		layout := tab.Layout
		if layout == "" {
			layout = DefaultLayout
		}
		cmds = append(cmds, fmt.Sprintf(
			"kitten goto-layout --match %s %s",
			shellQuote(MatchTabTitle(title)), shellQuote(layout),
		))
	}

	return cmds
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

// shellQuote wraps s in single quotes; embedded single quotes are escaped as '\”.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
