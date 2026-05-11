package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewRootCmd_HasExpectedSubcommands(t *testing.T) {
	cmd := newRootCmd()
	want := []string{"up", "down", "ls", "edit", "new", "delete", "debug", "capture", "local", "validate", "init"}
	subNames := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	for _, name := range want {
		if !subNames[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestNewRootCmd_HelpContainsSesh(t *testing.T) {
	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	// Help exits 0 but may return a special error; ignore it.
	_ = cmd.Execute()
	out := buf.String()
	if !strings.Contains(out, "sesh") {
		t.Errorf("help output missing 'sesh': %q", out)
	}
}

func TestNewRootCmd_VersionFlag(t *testing.T) {
	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--version"})
	_ = cmd.Execute()
	out := buf.String()
	if !strings.Contains(out, Version) {
		t.Errorf("version output should contain %q, got %q", Version, out)
	}
}
