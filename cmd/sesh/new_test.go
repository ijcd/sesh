package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewNewCmd_CreatesFile(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"myproject"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dest := filepath.Join(cfg, "sesh", "projects", "myproject.yml")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("project file not created: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "myproject") {
		t.Errorf("project file body should contain project name, got:\n%s", body)
	}
	if !strings.Contains(buf.String(), "created") {
		t.Errorf("expected 'created' in output, got %q", buf.String())
	}
}

func TestNewNewCmd_ErrorsWhenAlreadyExists(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "existing.yml"), []byte("driver: tmux\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newNewCmd()
	cmd.SetArgs([]string{"existing"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when project already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
}

func TestNewNewCmd_FromFlag(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcBody := "driver: tmux\ntabs:\n  - title: custom\n"
	if err := os.WriteFile(filepath.Join(projDir, "template.yml"), []byte(srcBody), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--from", "template", "derived"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dest := filepath.Join(projDir, "derived.yml")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("derived file not created: %v", err)
	}
	if string(data) != srcBody {
		t.Errorf("derived body = %q, want %q", string(data), srcBody)
	}
}
