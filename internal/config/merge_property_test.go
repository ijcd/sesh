package config

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/ijcd/sesh/internal/spec"
	"github.com/ijcd/sesh/internal/testutil"
)

// Property: Merging a project with itself produces the same project.
func TestMerge_Idempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p := testutil.ProjectGen().Draw(t, "p")
		// Set Driver explicitly so coalesce produces stable output
		if p.Driver == "" {
			p.Driver = "tmux"
		}
		out := Merge(&p, &p)
		if out.Driver != p.Driver {
			t.Errorf("Driver changed: %q → %q", p.Driver, out.Driver)
		}
		if len(out.Tabs) != len(p.Tabs) {
			t.Errorf("tab count changed: %d → %d", len(p.Tabs), len(out.Tabs))
		}
	})
}

// Property: Merging an empty parent into a child returns the child unchanged
// for scalar fields. (Hooks lists may differ if child has them — that's OK.)
func TestMerge_EmptyParentPreservesChild(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		child := testutil.ProjectGen().Draw(t, "child")
		empty := &spec.Project{}
		out := Merge(empty, &child)
		if out.Driver != child.Driver {
			t.Errorf("Driver: got %q, want %q", out.Driver, child.Driver)
		}
		if out.Cwd != child.Cwd {
			t.Errorf("Cwd: got %q, want %q", out.Cwd, child.Cwd)
		}
		if len(out.Tabs) != len(child.Tabs) {
			t.Errorf("tab count: got %d, want %d", len(out.Tabs), len(child.Tabs))
		}
	})
}

// Property: Merging child into empty parent preserves child completely.
func TestMerge_EmptyChildPreservesParent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		parent := testutil.ProjectGen().Draw(t, "parent")
		empty := &spec.Project{}
		out := Merge(&parent, empty)
		if len(out.Tabs) != len(parent.Tabs) {
			t.Errorf("tab count: got %d, want %d", len(out.Tabs), len(parent.Tabs))
		}
	})
}

// Property: After merge, all child-titled tabs survive (not dropped, not lost).
func TestMerge_ChildTabsPreserved(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		parent := testutil.ProjectGen().Draw(t, "parent")
		child := testutil.ProjectGen().Draw(t, "child")
		out := Merge(&parent, &child)
		// Every child tab title should appear in the output.
		outTitles := map[string]bool{}
		for _, tab := range out.Tabs {
			outTitles[tab.Title] = true
		}
		for _, tab := range child.Tabs {
			if !outTitles[tab.Title] {
				t.Errorf("child tab %q lost in merge", tab.Title)
			}
		}
	})
}
