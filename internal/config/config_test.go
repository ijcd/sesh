package config

import (
	"os"
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

func TestLoad_AppliesGlobalDefaults(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	// Write global config
	cfgDir := filepath.Join(base, "sesh")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yml"), []byte(`driver: kitty
editor: nvim
`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a project that omits driver
	projDir := filepath.Join(cfgDir, "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "p.yml"), []byte(`cwd: /tmp
tabs: [{title: shell}]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Load("p", []string{"kitty", "tmux"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Driver != "kitty" {
		t.Errorf("Driver = %q, want kitty (from global)", p.Driver)
	}
}

func TestLoad_ProjectDriverWinsOverGlobal(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	cfgDir := filepath.Join(base, "sesh")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, "config.yml"), []byte(`driver: kitty
`), 0o644)
	projDir := filepath.Join(cfgDir, "projects")
	os.MkdirAll(projDir, 0o755)
	os.WriteFile(filepath.Join(projDir, "p.yml"), []byte(`driver: tmux
cwd: /tmp
tabs: [{title: shell}]
`), 0o644)

	p, err := Load("p", []string{"kitty", "tmux"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Driver != "tmux" {
		t.Errorf("project's tmux should win over global's kitty, got %q", p.Driver)
	}
}
