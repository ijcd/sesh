package engine

import (
	"context"
	"testing"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/spec"
)

func newTestEngine() (*Engine, *mock.Driver) {
	md := mock.New("tmux")
	e := New()
	e.Register(md)
	return e, md
}

func TestUp_HappyPath(t *testing.T) {
	e, md := newTestEngine()
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
	}
	if err := e.Up(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}
	if len(md.UpCalls) != 1 {
		t.Errorf("driver.Up not called: %v", md.UpCalls)
	}
}

func TestUp_AttachExistingSilently(t *testing.T) {
	e, md := newTestEngine()
	md.StatusVal = drivers.StatusExists
	p := &spec.Project{
		Name: "x", Driver: "tmux",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
	}
	if err := e.Up(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}
	if len(md.UpCalls) != 0 {
		t.Errorf("driver.Up should not be called when attaching existing")
	}
}

func TestUp_ForceRecreates(t *testing.T) {
	e, md := newTestEngine()
	md.StatusVal = drivers.StatusExists
	p := &spec.Project{
		Name: "x", Driver: "tmux",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
	}
	if err := e.Up(context.Background(), p, true); err != nil {
		t.Fatal(err)
	}
	if len(md.DownCalls) != 1 || md.DownCalls[0] != "x" {
		t.Errorf("Down should have been called for force, got %v", md.DownCalls)
	}
	if len(md.UpCalls) != 1 {
		t.Errorf("Up should have been called after force-down")
	}
}

func TestUp_PreHookFailureAborts(t *testing.T) {
	e, md := newTestEngine()
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Hooks: spec.Hooks{Pre: []string{"false"}},
		Tabs:  []spec.Tab{{Title: "a", Cmd: "echo"}},
	}
	if err := e.Up(context.Background(), p, false); err == nil {
		t.Fatal("expected pre-hook abort")
	}
	if len(md.UpCalls) != 0 {
		t.Errorf("driver.Up should not run after failed pre")
	}
}

func TestUp_ContainmentEnforced(t *testing.T) {
	e, _ := newTestEngine()
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "x", Driver: "kitty",
			Panes: []spec.Pane{{Title: "p", Cmd: "y"}}}},
	}
	if err := e.Up(context.Background(), p, false); err == nil {
		t.Fatal("expected containment error")
	}
}

func TestUp_PreWindowPrependedToPaneCmd(t *testing.T) {
	e, md := newTestEngine()
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		PreWindow: []string{"direnv allow"},
		Tabs: []spec.Tab{{
			Title:     "dev",
			Driver:    "tmux",
			PreWindow: []string{"source .envrc"},
			Panes:     []spec.Pane{{Title: "p", Cmd: "overmind start"}},
		}},
	}
	if err := e.Up(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}
	sent := md.UpCalls[0]
	got := sent.Tabs[0].Panes[0].Cmd
	want := "direnv allow && source .envrc && overmind start"
	if got != want {
		t.Errorf("pane cmd = %q, want %q", got, want)
	}
}
