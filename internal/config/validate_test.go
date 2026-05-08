package config

import (
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func TestValidate_OK(t *testing.T) {
	p := &spec.Project{
		Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "shell"}, {Title: "dev", Cmd: "echo"}},
	}
	if errs := Validate(p, []string{"tmux"}); len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidate_RequiresTabTitle(t *testing.T) {
	p := &spec.Project{Driver: "tmux", Tabs: []spec.Tab{{Cmd: "x"}}}
	errs := Validate(p, []string{"tmux"})
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "tabs[0].title") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected required-title error, got %v", errs)
	}
}

func TestValidate_RequiresUniqueTabTitles(t *testing.T) {
	p := &spec.Project{Driver: "tmux", Tabs: []spec.Tab{{Title: "x"}, {Title: "x"}}}
	errs := Validate(p, []string{"tmux"})
	if len(errs) == 0 {
		t.Fatal("expected duplicate-title error")
	}
	if !strings.Contains(errs[0].Error(), "duplicate") {
		t.Errorf("error should mention duplicate: %v", errs[0])
	}
}

func TestValidate_RequiresPaneTitleAndCmd(t *testing.T) {
	p := &spec.Project{Driver: "tmux", Tabs: []spec.Tab{{Title: "dev", Panes: []spec.Pane{
		{Cmd: "x"},    // missing title
		{Title: "p2"}, // missing cmd
	}}}}
	errs := Validate(p, []string{"tmux"})
	if len(errs) < 2 {
		t.Errorf("expected 2 pane errors, got %v", errs)
	}
}

func TestValidate_TabCmdAndPanesMutuallyExclusive(t *testing.T) {
	p := &spec.Project{Driver: "tmux", Tabs: []spec.Tab{{
		Title: "x", Cmd: "echo", Panes: []spec.Pane{{Title: "p", Cmd: "y"}},
	}}}
	errs := Validate(p, []string{"tmux"})
	if len(errs) == 0 {
		t.Fatal("expected mutual-exclusion error")
	}
}

func TestValidate_DriverMustBeRegistered(t *testing.T) {
	p := &spec.Project{Driver: "kitty", Tabs: []spec.Tab{{Title: "shell"}}}
	errs := Validate(p, []string{"tmux"})
	if len(errs) == 0 {
		t.Fatal("expected unregistered-driver error")
	}
	if !strings.Contains(errs[0].Error(), "kitty") {
		t.Errorf("error should mention kitty: %v", errs[0])
	}
}

func TestValidate_DefaultsDriverWhenEmpty(t *testing.T) {
	p := &spec.Project{Tabs: []spec.Tab{{Title: "shell"}}}
	errs := Validate(p, []string{"tmux"})
	if len(errs) > 0 {
		t.Errorf("empty driver should default to tmux, got %v", errs)
	}
	if p.Driver != "tmux" {
		t.Errorf("driver not defaulted, got %q", p.Driver)
	}
}
