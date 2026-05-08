package engine

import (
	"fmt"

	"github.com/ijcd/sesh/internal/spec"
)

// Pair is a parent-driver/child-driver tuple. Empty Child = leaf cmd.
type Pair struct{ Parent, Child string }

// ValidPairs is the static matrix of allowed (parent, child) driver combos.
// v0.1 only ships the tmux driver, but we list future-driver pairs for forward
// compatibility — the engine errors if a pair is missing OR the driver isn't
// registered (the latter is checked separately by config.Validate).
var ValidPairs = map[Pair]bool{
	{Parent: "tmux", Child: "tmux"}:       true,
	{Parent: "tmux", Child: ""}:           true, // leaf
	{Parent: "kitty", Child: "kitty"}:     true,
	{Parent: "kitty", Child: "tmux"}:      true,
	{Parent: "kitty", Child: ""}:          true,
	{Parent: "wezterm", Child: "wezterm"}: true,
	{Parent: "wezterm", Child: "tmux"}:    true,
	{Parent: "wezterm", Child: ""}:        true,
}

// CheckContainment enforces parent/child driver compatibility.
// A tab with panes uses tab.Driver; a leaf tab (no panes) uses "" as child.
// Tab.Driver = "" inherits the project driver.
func CheckContainment(p *spec.Project) error {
	parent := p.Driver
	for i, tab := range p.Tabs {
		var child string
		if len(tab.Panes) > 0 {
			child = tab.Driver
			if child == "" {
				child = parent // inherit
			}
		} else {
			child = "" // leaf
		}
		if !ValidPairs[Pair{Parent: parent, Child: child}] {
			return fmt.Errorf("tabs[%d] %q: driver pair (project=%s, tab=%s) is not valid",
				i, tab.Title, parent, child)
		}
	}
	return nil
}
