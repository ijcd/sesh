// Package testutil provides shared test scaffolding: rapid generators and
// snapshot comparison helpers.
package testutil

import (
	"pgregory.net/rapid"

	"github.com/ijcd/sesh/internal/spec"
)

// SafeName generates a string suitable for tab/pane titles (no colons,
// reasonable length, ASCII).
func SafeName() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,15}`)
}

// SafeCmd generates a command string that won't cause shell-quoting drama.
func SafeCmd() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9 _.-]{0,40}`)
}

// SafeCwd generates an absolute-looking POSIX path.
func SafeCwd() *rapid.Generator[string] {
	return rapid.StringMatching(`/[a-z][a-z0-9_/-]{0,30}`)
}

// PaneGen generates a spec.Pane with a required title and required cmd.
func PaneGen() *rapid.Generator[spec.Pane] {
	return rapid.Custom(func(t *rapid.T) spec.Pane {
		return spec.Pane{
			Title: SafeName().Draw(t, "title"),
			Cmd:   SafeCmd().Draw(t, "cmd"),
			Cwd:   rapid.OneOf(rapid.Just(""), SafeCwd()).Draw(t, "cwd"),
		}
	})
}

// TabGen generates a spec.Tab — either leaf (cmd, no panes) or multi-pane.
func TabGen() *rapid.Generator[spec.Tab] {
	return rapid.Custom(func(t *rapid.T) spec.Tab {
		title := SafeName().Draw(t, "title")
		isLeaf := rapid.Bool().Draw(t, "isLeaf")
		if isLeaf {
			return spec.Tab{Title: title, Cmd: SafeCmd().Draw(t, "cmd")}
		}
		panes := rapid.SliceOfNDistinct(PaneGen(), 1, 4, func(p spec.Pane) string {
			return p.Title
		}).Draw(t, "panes")
		return spec.Tab{Title: title, Panes: panes}
	})
}

// ProjectGen generates a spec.Project with 1-5 distinct-titled tabs.
func ProjectGen() *rapid.Generator[spec.Project] {
	return rapid.Custom(func(t *rapid.T) spec.Project {
		return spec.Project{
			Name:   SafeName().Draw(t, "name"),
			Driver: rapid.SampledFrom([]string{"tmux", "kitty"}).Draw(t, "driver"),
			Cwd:    SafeCwd().Draw(t, "cwd"),
			Tabs: rapid.SliceOfNDistinct(TabGen(), 1, 5, func(tab spec.Tab) string {
				return tab.Title
			}).Draw(t, "tabs"),
		}
	})
}
