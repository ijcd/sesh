package config

import (
	"path/filepath"
	"testing"
)

func TestLoad_AppliesPipeline(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "config")
	t.Setenv("XDG_CONFIG_HOME", base)
	p, err := Load("leaf", []string{"tmux"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "leaf" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Driver != "tmux" {
		t.Errorf("Driver = %q", p.Driver)
	}
	if len(p.Hooks.Pre) != 3 {
		t.Errorf("Hooks.Pre = %v (extends chain not applied)", p.Hooks.Pre)
	}
}

func TestLoad_ProjectNotFound(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	_, err := Load("nope", []string{"tmux"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFromPath_LocalDotSesh(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "config")
	p, err := LoadFromPath(filepath.Join(base, "projects", "example.yml"), []string{"tmux"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "example" || len(p.Tabs) != 2 {
		t.Errorf("got %+v", p)
	}
}
