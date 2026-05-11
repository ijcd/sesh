package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/engine"
)

func TestNewLocalCmd_MissingSeshYml(t *testing.T) {
	dir := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	// Change working directory for the duration of this test.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newLocalCmd(e)
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error when .sesh.yml is missing")
	}
	if !strings.Contains(err.Error(), ".sesh.yml") {
		t.Errorf("error should mention .sesh.yml, got: %v", err)
	}
}

func TestNewLocalCmd_RunsFromSeshYml(t *testing.T) {
	dir := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_LISTEN_ON", "")

	seshYml := filepath.Join(dir, ".sesh.yml")
	content := "driver: tmux\nattach: false\ntabs:\n  - title: shell\n"
	if err := os.WriteFile(seshYml, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	md := mock.New("tmux")
	e := engine.New()
	e.Register(md)

	cmd := newLocalCmd(e)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(md.UpCalls) != 1 {
		t.Errorf("expected 1 Up call, got %d", len(md.UpCalls))
	}
}
