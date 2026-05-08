package config

import (
	"path/filepath"
	"testing"
)

func TestLoadFile_Minimal(t *testing.T) {
	p, err := LoadFile(filepath.Join("..", "..", "testdata", "config", "projects", "example.yml"))
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if p.Name != "example" {
		t.Errorf("Name = %q, want example", p.Name)
	}
	if len(p.Tabs) != 2 {
		t.Errorf("len(Tabs) = %d, want 2", len(p.Tabs))
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("nonexistent.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFile_ParseError(t *testing.T) {
	_, err := LoadFile(filepath.Join("..", "..", "testdata", "config", "broken.yml"))
	if err == nil {
		t.Fatal("expected error for unparseable file")
	}
}

func TestResolveProjectPath_HomeDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	got, err := ResolveProjectPath("foo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "sesh", "projects", "foo.yml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveProjectPath_XDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, err := ResolveProjectPath("foo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "sesh", "projects", "foo.yml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
