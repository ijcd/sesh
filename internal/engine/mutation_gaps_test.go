package engine

// Tests written in response to LIVED mutations surfaced by gremlins.
// Each test name references the mutation location.

import (
	"context"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/spec"
)

// containment.go:34 — len(tab.Panes) > 0 boundary.
// CONDITIONALS_BOUNDARY mutation survived: >= 0 would always be true,
// so a zero-pane tab would be treated as needing a child driver.

func TestCheckContainment_ZeroPaneTabIsLeaf(t *testing.T) {
	// A tab with 0 panes is a leaf: child = "" regardless of tab.Driver.
	p := &spec.Project{
		Driver: "kitty",
		Tabs: []spec.Tab{
			// tab.Driver set, but 0 panes → must be treated as leaf ("" child)
			{Title: "x", Driver: "kitty"},
		},
	}
	// (kitty, "") is a valid pair; this should pass.
	if err := CheckContainment(p); err != nil {
		t.Errorf("zero-pane tab should be treated as leaf (pair kitty/\"\"), got: %v", err)
	}
}

func TestCheckContainment_OnePaneTabUsesTabDriver(t *testing.T) {
	// A tab with exactly 1 pane is NOT a leaf: child = tab.Driver.
	p := &spec.Project{
		Driver: "kitty",
		Tabs: []spec.Tab{
			{Title: "x", Driver: "tmux", Panes: []spec.Pane{{Title: "p", Cmd: "y"}}},
		},
	}
	// (kitty, tmux) is valid.
	if err := CheckContainment(p); err != nil {
		t.Errorf("one-pane tab with valid pair should pass, got: %v", err)
	}
}

// up.go:41 — cross-driver Cwd inheritance.
// Mutation: negating inner.Cwd=="" survived.
// Test: tab with own Cwd keeps it; tab without Cwd inherits from project.

func TestTransformCrossDriverTabs_TabCwdKept(t *testing.T) {
	tmuxDrv := mock.New("tmux")
	tmuxDrv.AttachCommandVal = "tmux attach -t proj-dev"
	e := New()
	e.Register(tmuxDrv)
	e.Register(mock.New("kitty"))

	p := &spec.Project{
		Name: "proj", Driver: "kitty", Cwd: "/project",
		Tabs: []spec.Tab{
			{Title: "dev", Driver: "tmux", Cwd: "/custom",
				Panes: []spec.Pane{{Title: "p", Cmd: "x"}}},
		},
	}
	_, err := e.dispatchCrossDriverTabs(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	inner := tmuxDrv.UpCalls[0]
	if inner.Cwd != "/custom" {
		t.Errorf("inner.Cwd = %q, want /custom (tab's own Cwd)", inner.Cwd)
	}
}

func TestTransformCrossDriverTabs_TabCwdInheritsProjectWhenEmpty(t *testing.T) {
	tmuxDrv := mock.New("tmux")
	tmuxDrv.AttachCommandVal = "tmux attach -t proj-dev"
	e := New()
	e.Register(tmuxDrv)
	e.Register(mock.New("kitty"))

	p := &spec.Project{
		Name: "proj", Driver: "kitty", Cwd: "/project",
		Tabs: []spec.Tab{
			{Title: "dev", Driver: "tmux", Cwd: "", // empty: should inherit
				Panes: []spec.Pane{{Title: "p", Cmd: "x"}}},
		},
	}
	_, err := e.dispatchCrossDriverTabs(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	inner := tmuxDrv.UpCalls[0]
	if inner.Cwd != "/project" {
		t.Errorf("inner.Cwd = %q, want /project (inherited from project)", inner.Cwd)
	}
}

// up.go:105,108 — post/on_start hooks RunHooks called; failure propagates.
// Mutations on err!=nil checks for RunHooks survived because tests didn't
// assert that hook failures actually surface as errors from engine.Up.

func TestUp_PostHookFailureReturnsError(t *testing.T) {
	e, _ := newTestEngine()
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Hooks: spec.Hooks{Post: []string{"false"}},
		Tabs:  []spec.Tab{{Title: "a", Cmd: "echo"}},
	}
	if err := e.Up(context.Background(), p, false); err == nil {
		t.Fatal("expected error from failing post hook")
	}
}

func TestUp_OnStartHookFailureReturnsError(t *testing.T) {
	e, _ := newTestEngine()
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Hooks: spec.Hooks{OnStart: []string{"false"}},
		Tabs:  []spec.Tab{{Title: "a", Cmd: "echo"}},
	}
	if err := e.Up(context.Background(), p, false); err == nil {
		t.Fatal("expected error from failing on_start hook")
	}
}
