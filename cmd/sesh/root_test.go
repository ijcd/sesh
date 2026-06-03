package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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

// TestRoot_LuaPluginErrorDoesNotPanic confirms a broken user plugin file
// emits a warning but does not panic. Previously a syntax error in any
// ~/.config/sesh/plugins/*.lua killed every sesh command (incl. tier-1:
// `ls`, `edit`, `init`). The user plugin dir is rooted at $HOME, so we
// redirect HOME to a temp dir and drop a deliberately-broken .lua there.
func TestRoot_LuaPluginErrorDoesNotPanic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pluginDir := filepath.Join(home, ".config", "sesh", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Lua syntax error: `function (` with no closing.
	broken := []byte("function ( -- syntax error\n")
	if err := os.WriteFile(filepath.Join(pluginDir, "broken.lua"), broken, 0o644); err != nil {
		t.Fatal(err)
	}

	// Capture stderr to verify the warning is emitted.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	done := make(chan struct{})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("newRootCmd panicked on broken lua plugin: %v", r)
		}
	}()

	cmd := newRootCmd()
	_ = w.Close()
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	<-done

	if cmd == nil {
		t.Fatal("newRootCmd returned nil")
	}
	out := buf.String()
	if !strings.Contains(out, "lua plugin load failed") {
		t.Errorf("expected stderr warning about lua load failure, got %q", out)
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
