package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewInitCmd_Zsh(t *testing.T) {
	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"zsh"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// zsh snippet should contain a shell-specific marker.
	if !strings.Contains(out, "sesh") {
		t.Errorf("zsh init output missing 'sesh': %q", out)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output for zsh init")
	}
}

func TestNewInitCmd_Bash(t *testing.T) {
	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"bash"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if len(out) == 0 {
		t.Error("expected non-empty output for bash init")
	}
}

func TestNewInitCmd_Fish(t *testing.T) {
	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"fish"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if len(out) == 0 {
		t.Error("expected non-empty output for fish init")
	}
}

func TestNewInitCmd_UnknownShellErrors(t *testing.T) {
	cmd := newInitCmd()
	cmd.SetArgs([]string{"powershell"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown shell")
	}
	if !strings.Contains(err.Error(), "unknown init target") {
		t.Errorf("error should mention 'unknown init target', got: %v", err)
	}
	if !strings.Contains(err.Error(), "emacs") {
		t.Errorf("error should list emacs in valid targets, got: %v", err)
	}
}

func TestNewInitCmd_Emacs(t *testing.T) {
	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"emacs"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"(defun sesh-open-project",
		"(defun sesh-close-project",
		"(provide 'sesh)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("emacs init output missing %q", want)
		}
	}
}
