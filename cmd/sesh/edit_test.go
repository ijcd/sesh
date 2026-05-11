package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEditCmd_OpensExistingFile(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	// "true" is a no-op program that exits 0 — works as a mock editor.
	t.Setenv("EDITOR", "true")

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "myproj.yml"), []byte("driver: tmux\ntabs:\n  - title: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newEditCmd()
	cmd.SetArgs([]string{"myproj"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewEditCmd_ErrorsWhenProjectMissing(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("EDITOR", "true")

	cmd := newEditCmd()
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing project")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' in error, got: %v", err)
	}
}
