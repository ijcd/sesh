package init

import (
	"strings"
	"testing"
)

func TestRender_Zsh(t *testing.T) {
	s, err := Render("zsh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "chpwd_functions") {
		t.Errorf("zsh script should reference chpwd_functions")
	}
	if !strings.Contains(s, "sesh: project here") {
		t.Errorf("zsh script should print discovery message")
	}
}

func TestRender_Bash(t *testing.T) {
	s, err := Render("bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "PROMPT_COMMAND") {
		t.Errorf("bash script should set PROMPT_COMMAND")
	}
}

func TestRender_Fish(t *testing.T) {
	s, err := Render("fish")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "--on-variable PWD") {
		t.Errorf("fish script should hook on PWD changes")
	}
}

func TestRender_UnknownShell(t *testing.T) {
	_, err := Render("ksh")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ksh") {
		t.Errorf("error should name the unknown shell: %v", err)
	}
	for _, want := range []string{"zsh", "bash", "fish"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should list supported shell %q: %v", want, err)
		}
	}
}
