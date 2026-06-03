package xdg

import (
	"path/filepath"
	"testing"
)

func TestConfigHome_EnvSet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	got, err := ConfigHome()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/custom/config" {
		t.Errorf("ConfigHome() = %q, want %q", got, "/custom/config")
	}
}

func TestConfigHome_EnvUnsetFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/fake/home")
	got, err := ConfigHome()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/fake/home", ".config")
	if got != want {
		t.Errorf("ConfigHome() = %q, want %q", got, want)
	}
}

func TestConfigHome_NoHomeReturnsError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if _, err := ConfigHome(); err == nil {
		t.Fatal("expected error when XDG_CONFIG_HOME and HOME are both empty")
	}
}

func TestStateHome_EnvSet(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	got, err := StateHome()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/custom/state" {
		t.Errorf("StateHome() = %q, want %q", got, "/custom/state")
	}
}

func TestStateHome_EnvUnsetFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/fake/home")
	got, err := StateHome()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/fake/home", ".local", "state")
	if got != want {
		t.Errorf("StateHome() = %q, want %q", got, want)
	}
}

func TestStateHome_NoHomeReturnsError(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")
	if _, err := StateHome(); err == nil {
		t.Fatal("expected error when XDG_STATE_HOME and HOME are both empty")
	}
}
