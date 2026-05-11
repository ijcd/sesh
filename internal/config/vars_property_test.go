package config

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/ijcd/sesh/internal/spec"
)

// Property: ExpandVars with no ${...} in any field is identity.
func TestExpandVars_NoVarsIsIdentity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cwd := rapid.StringMatching(`/[a-z]{1,10}`).Draw(t, "cwd")
		cmd := rapid.StringMatching(`[a-z ]{1,20}`).Draw(t, "cmd")
		p := &spec.Project{
			Cwd:  cwd,
			Tabs: []spec.Tab{{Title: "x", Cmd: cmd}},
		}
		if err := ExpandVars(p, nil); err != nil {
			t.Fatal(err)
		}
		if p.Cwd != cwd {
			t.Errorf("Cwd changed: %q → %q", cwd, p.Cwd)
		}
		if p.Tabs[0].Cmd != cmd {
			t.Errorf("Cmd changed: %q → %q", cmd, p.Tabs[0].Cmd)
		}
	})
}

// Property: $${NAME} always renders as ${NAME} regardless of vars.
func TestExpandVars_EscapeIsLiteral(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Z][A-Z0-9_]{0,8}`).Draw(t, "name")
		prefix := rapid.StringMatching(`[a-z]{1,5}`).Draw(t, "prefix")
		suffix := rapid.StringMatching(`[a-z]{1,5}`).Draw(t, "suffix")
		cmd := prefix + " $${" + name + "} " + suffix
		p := &spec.Project{
			Vars: map[string]string{name: "REPLACED"},
			Tabs: []spec.Tab{{Title: "x", Cmd: cmd}},
		}
		if err := ExpandVars(p, nil); err != nil {
			t.Fatal(err)
		}
		want := prefix + " ${" + name + "} " + suffix
		if p.Tabs[0].Cmd != want {
			t.Errorf("got %q, want %q", p.Tabs[0].Cmd, want)
		}
		if strings.Contains(p.Tabs[0].Cmd, "REPLACED") {
			t.Errorf("escape was substituted: %q", p.Tabs[0].Cmd)
		}
	})
}
