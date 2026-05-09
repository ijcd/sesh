package main

import (
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func TestAttachArgs_OutsideTmux(t *testing.T) {
	p := &spec.Project{Name: "demo"}
	args := attachArgs(p, false)
	want := []string{"tmux", "attach-session", "-t", "demo"}
	if !stringsEqual(args, want) {
		t.Errorf("got %v, want %v", args, want)
	}
}

func TestAttachArgs_InsideTmux(t *testing.T) {
	p := &spec.Project{Name: "demo"}
	args := attachArgs(p, true)
	want := []string{"tmux", "switch-client", "-t", "demo"}
	if !stringsEqual(args, want) {
		t.Errorf("got %v, want %v", args, want)
	}
}

func TestAttachArgs_CustomSession(t *testing.T) {
	p := &spec.Project{Name: "demo", Session: "custom"}
	args := attachArgs(p, false)
	want := []string{"tmux", "attach-session", "-t", "custom"}
	if !stringsEqual(args, want) {
		t.Errorf("got %v, want %v", args, want)
	}
}

func TestAttachArgs_SlugsName(t *testing.T) {
	p := &spec.Project{Name: "My Project"}
	args := attachArgs(p, false)
	want := []string{"tmux", "attach-session", "-t", "my-project"}
	if !stringsEqual(args, want) {
		t.Errorf("got %v, want %v", args, want)
	}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
