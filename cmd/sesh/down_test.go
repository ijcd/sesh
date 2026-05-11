package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/engine"
)

func TestNewDownCmd_StopsExistingProject(t *testing.T) {
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
	e := engine.New()
	e.Register(md)

	cmd := newDownCmd(e)
	cmd.SetArgs([]string{"demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(md.DownCalls) != 1 {
		t.Errorf("expected 1 Down call, got %d", len(md.DownCalls))
	}
}

func TestNewDownCmd_MissingProject(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newDownCmd(e)
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestNewDownCmd_RequiresOneArg(t *testing.T) {
	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newDownCmd(e)
	// No arguments provided.
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no project name given")
	}
}
