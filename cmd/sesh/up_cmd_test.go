package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/engine"
)

func TestNewUpCmd_LaunchesProject(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	// Ensure we're not inside tmux — avoids attachToTmux exec call.
	t.Setenv("TMUX", "")
	// Ensure not inside kitty — avoids kitty validation.
	t.Setenv("KITTY_LISTEN_ON", "")

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// attach: false so we don't try to exec tmux attach.
	content := "driver: tmux\nattach: false\ntabs:\n  - title: shell\n"
	if err := os.WriteFile(filepath.Join(projDir, "demo.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	md := mock.New("tmux")
	e := engine.New()
	e.Register(md)

	cmd := newUpCmd(e)
	cmd.SetArgs([]string{"demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(md.UpCalls) != 1 {
		t.Errorf("expected 1 Up call, got %d", len(md.UpCalls))
	}
	if md.UpCalls[0].Name != "demo" {
		t.Errorf("Up called with wrong name: %q", md.UpCalls[0].Name)
	}
}

func TestNewUpCmd_ForceFlag_DownsFirst(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_LISTEN_ON", "")

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "driver: tmux\nattach: false\ntabs:\n  - title: shell\n"
	if err := os.WriteFile(filepath.Join(projDir, "demo.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	md := mock.New("tmux")
	md.StatusVal = drivers.StatusExists // session already exists
	e := engine.New()
	e.Register(md)

	cmd := newUpCmd(e)
	cmd.SetArgs([]string{"--force", "demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(md.DownCalls) != 1 {
		t.Errorf("expected 1 Down call (force), got %d", len(md.DownCalls))
	}
	if len(md.UpCalls) != 1 {
		t.Errorf("expected 1 Up call after force-down, got %d", len(md.UpCalls))
	}
}

func TestNewUpCmd_MissingProject(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newUpCmd(e)
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestNewUpCmd_NoArgNoEnvErrors(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("SESH_PROJECT", "")

	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newUpCmd(e)
	// No args, no env — should fail.
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no project name provided")
	}
}
