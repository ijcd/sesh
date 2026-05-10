package kitty

import (
	"fmt"
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
	cwd := tab.Cwd
	if cwd == "" {
		cwd = p.Cwd
	}
	var sb strings.Builder
	sb.WriteString("kitten launch --type=tab")
	sb.WriteString(" --tab-title=" + shellQuote(title))
	sb.WriteString(" --cwd=" + shellQuote(cwd))
	if tab.Cmd != "" {
		// Wrap the user cmd in sh -c so compound commands (`&&`, pipes,
		// redirection) and quoted arguments work. Direct exec via -- argv
		// would split on whitespace and not interpret shell syntax.
		sb.WriteString(" --hold -- /bin/sh -c " + shellQuote(tab.Cmd))
	}
	return []string{sb.String()}
}

// shellQuote wraps s in single quotes; embedded single quotes are escaped as '\”.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
