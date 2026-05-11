package testutil

import (
	"testing"

	"pgregory.net/rapid"
)

func TestProjectGen_NeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p := ProjectGen().Draw(t, "p")
		if len(p.Tabs) == 0 {
			t.Fatal("project should have at least one tab")
		}
		for _, tab := range p.Tabs {
			if tab.Title == "" {
				t.Fatal("tab title should be non-empty")
			}
			for _, pane := range tab.Panes {
				if pane.Title == "" || pane.Cmd == "" {
					t.Fatal("pane title and cmd should be non-empty")
				}
			}
		}
	})
}
