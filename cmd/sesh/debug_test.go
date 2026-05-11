package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/engine"
)

func TestNewDebugCmd_PrintsSpec(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "demo.yml"), []byte("driver: tmux\ntabs:\n  - title: shell\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	md := mock.New("tmux")
	md.DryRunVal = []string{"tmux new-session -d -s demo"}

	e := engine.New()
	e.Register(md)

	cmd := newDebugCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// Should contain the YAML spec and the dry-run commands.
	if !strings.Contains(out, "tmux") {
		t.Errorf("expected driver/commands in output, got %q", out)
	}
}

func TestNewDebugCmd_CommandsOnlyFlag(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "demo.yml"), []byte("driver: tmux\ntabs:\n  - title: shell\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	md := mock.New("tmux")
	md.DryRunVal = []string{"tmux new-session -d -s demo"}

	e := engine.New()
	e.Register(md)

	cmd := newDebugCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--commands-only", "demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "tmux new-session") {
		t.Errorf("expected dry-run command in output, got %q", out)
	}
	// The YAML spec separator should NOT be present.
	if strings.Contains(out, "--- commands ---") {
		t.Errorf("commands-only should not include separator, got %q", out)
	}
}

func TestNewDebugCmd_MissingProject(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newDebugCmd(e)
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}
