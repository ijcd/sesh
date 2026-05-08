package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveExtends_Chain(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join("..", "..", "testdata", "config"))
	leafPath := filepath.Join("..", "..", "testdata", "config", "projects", "leaf.yml")
	leaf, err := LoadFile(leafPath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResolveExtends(leaf, leafPath)
	if err != nil {
		t.Fatal(err)
	}
	// Hooks.Pre order: base → middle → leaf (because ResolveExtends returns the
	// merged result, with parent-then-child append at every step).
	want := []string{"base-pre", "middle-pre", "leaf-pre"}
	got := out.Hooks.Pre
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("Hooks.Pre = %v, want %v", got, want)
	}
	// Tabs: alpha (from base), beta (from middle), gamma (from leaf) — none share titles.
	titles := []string{}
	for _, tb := range out.Tabs {
		titles = append(titles, tb.Title)
	}
	if len(titles) != 3 || titles[0] != "alpha" || titles[1] != "beta" || titles[2] != "gamma" {
		t.Errorf("titles = %v", titles)
	}
	// Driver inherited from base
	if out.Driver != "tmux" {
		t.Errorf("Driver = %q, want tmux", out.Driver)
	}
	// Extends field cleared on resolved spec
	if out.Extends != "" {
		t.Errorf("Extends should be cleared, got %q", out.Extends)
	}
}

func TestResolveExtends_Cycle(t *testing.T) {
	aPath := filepath.Join("..", "..", "testdata", "config", "cycle", "a.yml")
	a, err := LoadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ResolveExtends(a, aPath)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle: %v", err)
	}
}

func TestResolveExtends_NoExtends(t *testing.T) {
	p, err := LoadFile(filepath.Join("..", "..", "testdata", "config", "projects", "example.yml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResolveExtends(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if out != p {
		t.Errorf("ResolveExtends should return input unchanged when no extends")
	}
}
