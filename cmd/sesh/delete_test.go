package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDeleteCmd_DeletesWithYesFlag(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projDir, "todelete.yml")
	if err := os.WriteFile(path, []byte("driver: tmux\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newDeleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--yes", "todelete"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("project file should be deleted")
	}
	if !strings.Contains(buf.String(), "deleted") {
		t.Errorf("expected 'deleted' in output, got %q", buf.String())
	}
}

func TestNewDeleteCmd_ErrorsWhenNotFound(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	cmd := newDeleteCmd()
	cmd.SetArgs([]string{"--yes", "nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing project")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestNewDeleteCmd_AbortsWithoutYes(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projDir, "keep.yml")
	if err := os.WriteFile(path, []byte("driver: tmux\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Provide "n" on stdin so the prompt rejects.
	cmd := newDeleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs([]string{"keep"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Error("project file should still exist after abort")
	}
	if !strings.Contains(buf.String(), "aborted") {
		t.Errorf("expected 'aborted' in output, got %q", buf.String())
	}
}
