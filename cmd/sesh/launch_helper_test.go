package main

import (
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func TestNeedsLaunch_InKittyNoFlag(t *testing.T) {
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	p := &spec.Project{Driver: "kitty"}
	if needsLaunch(p, false) {
		t.Error("in-kitty + no flag → should not launch")
	}
}

func TestNeedsLaunch_InKittyWithFlag(t *testing.T) {
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	p := &spec.Project{Driver: "kitty"}
	if needsLaunch(p, true) {
		t.Error("in-kitty + flag → should be no-op (use existing kitty)")
	}
}

func TestNeedsLaunch_NotInKittyNoFlag(t *testing.T) {
	t.Setenv("KITTY_LISTEN_ON", "")
	p := &spec.Project{Driver: "kitty"}
	if needsLaunch(p, false) {
		t.Error("not-in-kitty + no flag should error elsewhere, not launch")
	}
}

func TestNeedsLaunch_NotInKittyWithFlag(t *testing.T) {
	t.Setenv("KITTY_LISTEN_ON", "")
	p := &spec.Project{Driver: "kitty"}
	if !needsLaunch(p, true) {
		t.Error("not-in-kitty + flag → should launch")
	}
}

func TestNeedsLaunch_NonKittyDriver(t *testing.T) {
	t.Setenv("KITTY_LISTEN_ON", "")
	p := &spec.Project{Driver: "tmux"}
	if needsLaunch(p, true) {
		t.Error("tmux project → never launches kitty")
	}
}
