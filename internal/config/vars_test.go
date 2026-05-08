package config

import (
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func TestExpandVars_FromMap(t *testing.T) {
	p := &spec.Project{
		Vars: map[string]string{"DB": "mydb"},
		Tabs: []spec.Tab{{Title: "db", Cmd: "psql ${DB}"}},
	}
	if err := ExpandVars(p, nil); err != nil {
		t.Fatal(err)
	}
	if p.Tabs[0].Cmd != "psql mydb" {
		t.Errorf("got %q", p.Tabs[0].Cmd)
	}
}

func TestExpandVars_FromEnv(t *testing.T) {
	p := &spec.Project{
		Tabs: []spec.Tab{{Title: "x", Cmd: "echo ${USER_FOO}"}},
	}
	if err := ExpandVars(p, map[string]string{"USER_FOO": "bar"}); err != nil {
		t.Fatal(err)
	}
	if p.Tabs[0].Cmd != "echo bar" {
		t.Errorf("got %q", p.Tabs[0].Cmd)
	}
}

func TestExpandVars_VarsBeatEnv(t *testing.T) {
	p := &spec.Project{
		Vars: map[string]string{"X": "from-vars"},
		Tabs: []spec.Tab{{Title: "x", Cmd: "${X}"}},
	}
	if err := ExpandVars(p, map[string]string{"X": "from-env"}); err != nil {
		t.Fatal(err)
	}
	if p.Tabs[0].Cmd != "from-vars" {
		t.Errorf("got %q", p.Tabs[0].Cmd)
	}
}

func TestExpandVars_Escape(t *testing.T) {
	p := &spec.Project{
		Tabs: []spec.Tab{{Title: "x", Cmd: "echo $${LITERAL}"}},
	}
	if err := ExpandVars(p, nil); err != nil {
		t.Fatal(err)
	}
	if p.Tabs[0].Cmd != "echo ${LITERAL}" {
		t.Errorf("got %q", p.Tabs[0].Cmd)
	}
}

func TestExpandVars_Undefined(t *testing.T) {
	p := &spec.Project{
		Name: "demo",
		Tabs: []spec.Tab{{Title: "x", Cmd: "echo ${MISSING}"}},
	}
	err := ExpandVars(p, nil)
	if err == nil {
		t.Fatal("expected error for undefined var")
	}
	if !strings.Contains(err.Error(), "MISSING") {
		t.Errorf("error should name MISSING: %v", err)
	}
	if !strings.Contains(err.Error(), "tabs[0].cmd") {
		t.Errorf("error should locate the offending key: %v", err)
	}
}

func TestExpandVars_RecursesIntoPanesAndHooksAndCwd(t *testing.T) {
	p := &spec.Project{
		Vars:      map[string]string{"A": "1", "B": "2"},
		Cwd:       "/x/${A}",
		Hooks:     spec.Hooks{Pre: []string{"echo ${B}"}},
		PreWindow: []string{"set ${A}"},
		Tabs: []spec.Tab{{
			Title: "t", Cwd: "/y/${A}",
			PreWindow: []string{"x ${B}"},
			Panes:     []spec.Pane{{Title: "p", Cmd: "z ${B}", Cwd: "/z/${A}"}},
		}},
	}
	if err := ExpandVars(p, nil); err != nil {
		t.Fatal(err)
	}
	if p.Cwd != "/x/1" || p.Hooks.Pre[0] != "echo 2" || p.PreWindow[0] != "set 1" {
		t.Errorf("top-level not expanded: %+v", p)
	}
	if p.Tabs[0].Cwd != "/y/1" || p.Tabs[0].PreWindow[0] != "x 2" {
		t.Errorf("tab-level not expanded: %+v", p.Tabs[0])
	}
	if p.Tabs[0].Panes[0].Cmd != "z 2" || p.Tabs[0].Panes[0].Cwd != "/z/1" {
		t.Errorf("pane-level not expanded: %+v", p.Tabs[0].Panes[0])
	}
}
