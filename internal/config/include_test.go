package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInclude_Scalar(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "config")
	t.Setenv("XDG_CONFIG_HOME", base)

	leafPath := filepath.Join(base, "sesh", "projects", "leaf.yml")
	leaf, err := LoadFile(leafPath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResolveInclude(leaf, leafPath)
	if err != nil {
		t.Fatal(err)
	}
	// leaf includes "middle" which includes "base"
	if len(out.Hooks.Pre) != 3 {
		t.Errorf("Hooks.Pre = %v, want 3 entries (base, middle, leaf)", out.Hooks.Pre)
	}
	if len(out.Include) != 0 {
		t.Errorf("Include should be cleared after resolve, got %v", out.Include)
	}
}

func TestResolveInclude_List(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "config")
	t.Setenv("XDG_CONFIG_HOME", base)

	p, err := LoadFile(filepath.Join(base, "sesh", "projects", "multi-include.yml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResolveInclude(p, filepath.Join(base, "sesh", "projects", "multi-include.yml"))
	if err != nil {
		t.Fatal(err)
	}
	// Pre order: base ("base-pre"), then direnv ("direnv allow"), then notify (none),
	// then leaf body ("leaf-pre").
	want := []string{"base-pre", "direnv allow", "leaf-pre"}
	got := []string(out.Hooks.Pre)
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("Hooks.Pre = %v, want %v", got, want)
	}
	if len(out.Hooks.OnStart) != 1 || out.Hooks.OnStart[0] != "notify start" {
		t.Errorf("Hooks.OnStart = %v", out.Hooks.OnStart)
	}
}

func TestResolveInclude_NoInclude(t *testing.T) {
	p, err := LoadFile(filepath.Join("..", "..", "testdata", "config", "projects", "example.yml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResolveInclude(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if out != p {
		t.Errorf("ResolveInclude should return input unchanged when no include")
	}
}

func TestResolveInclude_Cycle(t *testing.T) {
	aPath := filepath.Join("..", "..", "testdata", "config", "cycle", "a.yml")
	a, err := LoadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	// Bridge: rename Extends to Include for the cycle fixtures.
	a.Include = []string{a.Extends}
	a.Extends = ""
	_, err = ResolveInclude(a, aPath)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle: %v", err)
	}
}
