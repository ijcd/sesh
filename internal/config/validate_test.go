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

func TestValidate_DoesNotMutateDriverField(t *testing.T) {
	// Validate is pure — driver defaulting belongs to applyGlobalDefaults
	// (which runs earlier in the pipeline). Bare project with empty Driver
	// must surface as an unregistered-driver error, not be silently filled.
	p := &spec.Project{Tabs: []spec.Tab{{Title: "shell"}}}
	errs := Validate(p, []string{"tmux"})
	if p.Driver != "" {
		t.Errorf("Validate mutated Driver: got %q, want unchanged \"\"", p.Driver)
	}
	if len(errs) == 0 {
		t.Fatal("expected unregistered-driver error for empty Driver, got none")
	}
}

func TestApplyGlobalDefaults_DriverFallbackToTmux(t *testing.T) {
	// applyGlobalDefaults owns the hardcoded fallback when neither project
	// nor global config sets a driver.
	p := &spec.Project{Tabs: []spec.Tab{{Title: "shell"}}}
	applyGlobalDefaults(p, &Global{})
	if p.Driver != "tmux" {
		t.Errorf("applyGlobalDefaults: Driver = %q, want tmux", p.Driver)
	}
}

func TestValidate_RequiresAtLeastOneTab(t *testing.T) {
	p := &spec.Project{Driver: "tmux", Tabs: nil}
	errs := Validate(p, []string{"tmux"})
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "tabs") && strings.Contains(e.Error(), "at least one") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'tabs: at least one' error, got %v", errs)
	}
}

func TestValidate_ExtendsErrors(t *testing.T) {
	p := &spec.Project{
		Extends: "phoenix",
		Driver:  "tmux",
		Tabs:    []spec.Tab{{Title: "shell"}},
	}
	errs := Validate(p, []string{"tmux"})
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "extends:") && strings.Contains(e.Error(), "include:") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error mentioning extends: → include: migration; got %v", errs)
	}
}
