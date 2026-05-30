package config

import (
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/plugins"
	"github.com/ijcd/sesh/internal/spec"
)

func TestMergeApps_SameKeyChildScalarsWin(t *testing.T) {
	parent := &spec.Project{
		Tabs: []spec.Tab{{Title: "shell"}},
		Apps: []spec.App{{
			Plugin: "emacs", ID: "main", Optional: false,
			Raw: plugins.NewRawConfig(nil),
		}},
	}
	childRaw := plugins.NewRawConfig(nil)
	child := &spec.Project{
		Tabs: []spec.Tab{{Title: "shell"}},
		Apps: []spec.App{{
			Plugin: "emacs", ID: "main", Optional: true,
			Raw: childRaw,
		}},
	}
	out := Merge(parent, child)
	if len(out.Apps) != 1 {
		t.Fatalf("len(Apps) = %d, want 1 (%+v)", len(out.Apps), out.Apps)
	}
	if out.Apps[0].Optional != true {
		t.Errorf("Optional = %v, want true (child wins)", out.Apps[0].Optional)
	}
	if out.Apps[0].Raw != childRaw {
		t.Errorf("Raw not replaced by child's Raw")
	}
}

func TestMergeApps_NewKeysAppendAfterParent(t *testing.T) {
	parent := &spec.Project{
		Apps: []spec.App{spec.NewApp("emacs", "")},
	}
	child := &spec.Project{
		Apps: []spec.App{spec.NewApp("chrome", "")},
	}
	out := Merge(parent, child)
	if len(out.Apps) != 2 {
		t.Fatalf("len(Apps) = %d, want 2 (%+v)", len(out.Apps), out.Apps)
	}
	if out.Apps[0].Plugin != "emacs" || out.Apps[1].Plugin != "chrome" {
		t.Errorf("order = [%s,%s], want [emacs,chrome]",
			out.Apps[0].Plugin, out.Apps[1].Plugin)
	}
}

func TestMergeApps_DropRemovesInherited(t *testing.T) {
	parent := &spec.Project{
		Apps: []spec.App{spec.NewApp("emacs", "main")},
	}
	drop := spec.NewApp("emacs", "main")
	drop.Drop = true
	child := &spec.Project{Apps: []spec.App{drop}}
	out := Merge(parent, child)
	if len(out.Apps) != 0 {
		t.Errorf("len(Apps) = %d, want 0 (%+v)", len(out.Apps), out.Apps)
	}
}

func TestMergeApps_DropOnlyMatchesByKey(t *testing.T) {
	parent := &spec.Project{
		Apps: []spec.App{
			spec.NewApp("emacs", "main"),
			spec.NewApp("emacs", "extra"),
		},
	}
	drop := spec.NewApp("emacs", "main")
	drop.Drop = true
	child := &spec.Project{Apps: []spec.App{drop}}
	out := Merge(parent, child)
	if len(out.Apps) != 1 {
		t.Fatalf("len(Apps) = %d, want 1 (%+v)", len(out.Apps), out.Apps)
	}
	if out.Apps[0].Key() != "emacs#extra" {
		t.Errorf("survivor.Key() = %q, want emacs#extra", out.Apps[0].Key())
	}
}

func TestValidateApps_EmptyPluginErrors(t *testing.T) {
	p := &spec.Project{
		Driver: "tmux",
		Tabs:   []spec.Tab{{Title: "shell"}},
		Apps:   []spec.App{spec.NewApp("", "")},
	}
	errs := Validate(p, []string{"tmux"})
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "apps[0]") && strings.Contains(e.Error(), "plugin") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected apps[0].plugin error, got %v", errs)
	}
}

func TestValidateApps_DuplicateExplicitIdErrors(t *testing.T) {
	p := &spec.Project{
		Driver: "tmux",
		Tabs:   []spec.Tab{{Title: "shell"}},
		Apps: []spec.App{
			spec.NewApp("emacs", "main"),
			spec.NewApp("emacs", "main"),
		},
	}
	errs := Validate(p, []string{"tmux"})
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate") && strings.Contains(e.Error(), "main") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate-id error, got %v", errs)
	}
}

func TestValidateApps_RegistryNil_SkipsMembershipCheck(t *testing.T) {
	p := &spec.Project{
		Driver: "tmux",
		Tabs:   []spec.Tab{{Title: "shell"}},
		Apps:   []spec.App{spec.NewApp("not-registered", "")},
	}
	errs := ValidateWithRegistry(p, []string{"tmux"}, nil)
	for _, e := range errs {
		if strings.Contains(e.Error(), "not-registered") {
			t.Errorf("nil registry should skip membership check, got %v", e)
		}
	}
}

func TestValidateApps_RegistryWithUnregisteredPlugin_Errors(t *testing.T) {
	reg := plugins.NewRegistry()
	p := &spec.Project{
		Driver: "tmux",
		Tabs:   []spec.Tab{{Title: "shell"}},
		Apps:   []spec.App{spec.NewApp("emacs", "")},
	}
	errs := ValidateWithRegistry(p, []string{"tmux"}, reg)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "emacs") && strings.Contains(e.Error(), "registered") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unregistered-plugin error mentioning registered set, got %v", errs)
	}
}
