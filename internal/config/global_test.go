package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGlobal_Missing(t *testing.T) {
	g, err := LoadGlobal(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if g == nil {
		t.Fatal("expected non-nil empty Global")
	}
	if g.Driver != "" {
		t.Errorf("expected empty Driver, got %q", g.Driver)
	}
}

func TestLoadGlobal_OK(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(p, []byte(`driver: kitty
attach: false
vars:
  FOO: bar
`), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := LoadGlobal(p)
	if err != nil {
		t.Fatal(err)
	}
	if g.Driver != "kitty" {
		t.Errorf("Driver = %q", g.Driver)
	}
	if g.Attach == nil || *g.Attach != false {
		t.Errorf("Attach = %v, want false", g.Attach)
	}
	if g.Vars == nil || g.Vars["FOO"] != "bar" {
		t.Errorf("Vars[FOO] = %v, want bar", g.Vars["FOO"])
	}
}

func TestLoadGlobal_RejectsListField(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(p, []byte(`hooks:
  pre: ["direnv allow"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadGlobal(p)
	if err == nil {
		t.Fatal("expected error for list-typed top-level key")
	}
	if !strings.Contains(err.Error(), "hooks") || !strings.Contains(err.Error(), "list") {
		t.Errorf("error should mention list-typed field: %v", err)
	}
}

func TestLoadGlobal_RejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(p, []byte(`mystery: 42
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadGlobal(p)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "mystery") {
		t.Errorf("error should name unknown key: %v", err)
	}
}

func TestLoadGlobal_RejectsRemovedFields(t *testing.T) {
	testCases := []string{
		`editor: nvim`,
		`state_dir: ~/.local/state/sesh`,
		`launch:
  socket_dir: ~/.local/state/sesh/sockets`,
	}
	for _, content := range testCases {
		dir := t.TempDir()
		p := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadGlobal(p)
		if err == nil {
			t.Fatalf("expected error for removed field in: %s", content)
		}
		if !strings.Contains(err.Error(), "unknown key") {
			t.Errorf("error should be unknown key error: %v", err)
		}
	}
}

func TestGlobalDefaultPath_HomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	p, err := GlobalDefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "sesh", "config.yml")
	if p != want {
		t.Errorf("got %q, want %q", p, want)
	}
}

func TestGlobalDefaultPath_XDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	p, err := GlobalDefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "sesh", "config.yml")
	if p != want {
		t.Errorf("got %q, want %q", p, want)
	}
}
