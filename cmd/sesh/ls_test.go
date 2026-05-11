package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLsCmd_EmptyDir(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	cmd := newLsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "no projects") {
		t.Errorf("expected empty-dir message, got %q", out)
	}
}

func TestNewLsCmd_ListsProjects(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha.yml", "beta.yml", "gamma.yml"} {
		if err := os.WriteFile(filepath.Join(projDir, name), []byte("driver: tmux\ntabs:\n  - title: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := newLsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestNewLsCmd_SkipsNonYml(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a .yml and a .txt; only the .yml should appear.
	if err := os.WriteFile(filepath.Join(projDir, "real.yml"), []byte("driver: tmux\ntabs:\n  - title: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "ignore.txt"), []byte("not a project"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newLsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "ignore") {
		t.Errorf("non-.yml file should not appear in output: %q", out)
	}
	if !strings.Contains(out, "real") {
		t.Errorf("expected 'real' in output: %q", out)
	}
}
