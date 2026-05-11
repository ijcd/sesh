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

func TestNewValidateCmd_ValidProject(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "valid.yml"), []byte("driver: tmux\ntabs:\n  - title: shell\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newValidateCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"valid"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok:") {
		t.Errorf("expected 'ok:' in output, got %q", out)
	}
	if !strings.Contains(out, "tmux") {
		t.Errorf("expected driver name in output, got %q", out)
	}
}

func TestNewValidateCmd_MissingProject(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newValidateCmd(e)
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestNewValidateCmd_ReportsTabCount(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "driver: tmux\ntabs:\n  - title: a\n  - title: b\n  - title: c\n"
	if err := os.WriteFile(filepath.Join(projDir, "multi.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newValidateCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"multi"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "3") {
		t.Errorf("expected tab count in output, got %q", buf.String())
	}
}
