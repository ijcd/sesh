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

func TestRender_Zsh_ContainsPreexecHook(t *testing.T) {
	s, err := Render("zsh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "__sesh_record_cmd") {
		t.Errorf("zsh script should define __sesh_record_cmd")
	}
	if !strings.Contains(s, "kitten @ set-user-vars") {
		t.Errorf("zsh script should call kitten @ set-user-vars")
	}
	if !strings.Contains(s, "preexec_functions") {
		t.Errorf("zsh script should append to preexec_functions")
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

func TestRender_Bash_ContainsPreexecHook(t *testing.T) {
	s, err := Render("bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "__sesh_record_cmd_bash") {
		t.Errorf("bash script should define __sesh_record_cmd_bash")
	}
	if !strings.Contains(s, "kitten @ set-user-vars") {
		t.Errorf("bash script should call kitten @ set-user-vars")
	}
	if !strings.Contains(s, "trap") {
		t.Errorf("bash script should set DEBUG trap")
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

func TestRender_Fish_ContainsPreexecHook(t *testing.T) {
	s, err := Render("fish")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "__sesh_record_cmd") {
		t.Errorf("fish script should define __sesh_record_cmd")
	}
	if !strings.Contains(s, "kitten @ set-user-vars") {
		t.Errorf("fish script should call kitten @ set-user-vars")
	}
	if !strings.Contains(s, "fish_preexec") {
		t.Errorf("fish script should use fish_preexec event")
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
	for _, want := range []string{"zsh", "bash", "fish", "emacs"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should list supported target %q: %v", want, err)
		}
	}
}

func TestRender_Emacs(t *testing.T) {
	s, err := Render("emacs")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"(defun sesh-open-project",
		"(defun sesh-close-project",
		"(provide 'sesh)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("emacs recipe missing %q", want)
		}
	}
}
