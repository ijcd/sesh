package main

import (
	"testing"
)

func TestResolveUpName_FromArg(t *testing.T) {
	t.Setenv("SESH_PROJECT", "")
	name, err := resolveUpName([]string{"myproject"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "myproject" {
		t.Errorf("got %q, want %q", name, "myproject")
	}
}

func TestResolveUpName_FromEnv(t *testing.T) {
	t.Setenv("SESH_PROJECT", "envproject")
	name, err := resolveUpName([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "envproject" {
		t.Errorf("got %q, want %q", name, "envproject")
	}
}

func TestResolveUpName_ArgWinsOverEnv(t *testing.T) {
	t.Setenv("SESH_PROJECT", "envproject")
	name, err := resolveUpName([]string{"argproject"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "argproject" {
		t.Errorf("arg should win over env: got %q, want %q", name, "argproject")
	}
}

func TestResolveUpName_NoArgNoEnvErrors(t *testing.T) {
	t.Setenv("SESH_PROJECT", "")
	_, err := resolveUpName([]string{})
	if err == nil {
		t.Fatal("expected error when no arg and no env")
	}
}
