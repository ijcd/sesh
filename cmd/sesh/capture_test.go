package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/spec"
)

func TestNewCaptureCmd_NoSession(t *testing.T) {
	// mock.Capture returns nil project → "no current tmux session"
	md := mock.New("tmux")
	e := engine.New()
	e.Register(md)

	cmd := newCaptureCmd(e)
	cmd.SetArgs([]string{"captured"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no session to capture")
	}
	if !strings.Contains(err.Error(), "no current tmux session") {
		t.Errorf("expected 'no current tmux session' in error, got: %v", err)
	}
}

func TestNewCaptureCmd_OutputsYAML(t *testing.T) {
	md := mock.New("tmux")
	md.CaptureVal = &spec.Project{
		Driver: "tmux",
		Tabs:   []spec.Tab{{Title: "shell"}},
	}

	e := engine.New()
	e.Register(md)

	cmd := newCaptureCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"myproject"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// The YAML output should contain the tab title.
	if !strings.Contains(out, "shell") {
		t.Errorf("expected YAML with tab 'shell', got %q", out)
	}
}

func TestNewCaptureCmd_NoDriverSupportsCapture(t *testing.T) {
	// Register a kitty driver only — capture only looks for tmux.
	md := mock.New("kitty")
	e := engine.New()
	e.Register(md)

	cmd := newCaptureCmd(e)
	cmd.SetArgs([]string{"captured"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no tmux driver registered")
	}
	if !strings.Contains(err.Error(), "no driver supports capture") {
		t.Errorf("expected 'no driver supports capture', got: %v", err)
	}
}
