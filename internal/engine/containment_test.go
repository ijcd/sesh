package engine

import (
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func TestCheckContainment_TmuxLeaf(t *testing.T) {
	p := &spec.Project{Driver: "tmux", Tabs: []spec.Tab{{Title: "x", Cmd: "echo"}}}
	if err := CheckContainment(p); err != nil {
		t.Errorf("expected ok, got %v", err)
	}
}

func TestCheckContainment_TmuxTmux(t *testing.T) {
	p := &spec.Project{Driver: "tmux", Tabs: []spec.Tab{{
		Title: "x", Driver: "tmux",
		Panes: []spec.Pane{{Title: "p", Cmd: "y"}},
	}}}
	if err := CheckContainment(p); err != nil {
		t.Errorf("expected ok, got %v", err)
	}
}

func TestCheckContainment_TmuxKitty_Invalid(t *testing.T) {
	p := &spec.Project{Driver: "tmux", Tabs: []spec.Tab{{
		Title: "x", Driver: "kitty",
		Panes: []spec.Pane{{Title: "p", Cmd: "y"}},
	}}}
	err := CheckContainment(p)
	if err == nil {
		t.Fatal("expected containment error")
	}
	if !strings.Contains(err.Error(), "kitty") {
		t.Errorf("error should mention child driver: %v", err)
	}
}

func TestCheckContainment_LeafNoDriverInherits(t *testing.T) {
	// tab has no panes, no driver — leaf — fine
	p := &spec.Project{Driver: "tmux", Tabs: []spec.Tab{{Title: "x"}}}
	if err := CheckContainment(p); err != nil {
		t.Errorf("expected ok for leaf, got %v", err)
	}
}
