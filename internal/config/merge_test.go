package config

import (
	"reflect"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func ptrBool(b bool) *bool { return &b }

func TestMerge_ScalarsChildWins(t *testing.T) {
	parent := &spec.Project{Cwd: "/p", Driver: "tmux", Session: "s"}
	child := &spec.Project{Cwd: "/c", Driver: "kitty"}
	out := Merge(parent, child)
	if out.Cwd != "/c" || out.Driver != "kitty" || out.Session != "s" {
		t.Errorf("scalar merge wrong: %+v", out)
	}
}

func TestMerge_AttachOverrideToFalse(t *testing.T) {
	parent := &spec.Project{Attach: ptrBool(true)}
	child := &spec.Project{Attach: ptrBool(false)}
	out := Merge(parent, child)
	if *out.Attach != false {
		t.Errorf("Attach = %v, want false", *out.Attach)
	}
}

func TestMerge_VarsDeepMerge(t *testing.T) {
	parent := &spec.Project{Vars: map[string]string{"A": "1", "B": "2"}}
	child := &spec.Project{Vars: map[string]string{"B": "X", "C": "3"}}
	out := Merge(parent, child)
	want := map[string]string{"A": "1", "B": "X", "C": "3"}
	if !reflect.DeepEqual(out.Vars, want) {
		t.Errorf("Vars = %v, want %v", out.Vars, want)
	}
}

func TestMerge_HookListsAppend(t *testing.T) {
	parent := &spec.Project{Hooks: spec.Hooks{Pre: []string{"a"}, Post: []string{"x"}}}
	child := &spec.Project{Hooks: spec.Hooks{Pre: []string{"b"}, OnStart: []string{"y"}}}
	out := Merge(parent, child)
	if !reflect.DeepEqual(out.Hooks.Pre, []string{"a", "b"}) {
		t.Errorf("Pre = %v", out.Hooks.Pre)
	}
	if !reflect.DeepEqual(out.Hooks.Post, []string{"x"}) {
		t.Errorf("Post = %v", out.Hooks.Post)
	}
	if !reflect.DeepEqual(out.Hooks.OnStart, []string{"y"}) {
		t.Errorf("OnStart = %v", out.Hooks.OnStart)
	}
}

func TestMerge_PreWindowAppends(t *testing.T) {
	parent := &spec.Project{PreWindow: []string{"a"}}
	child := &spec.Project{PreWindow: []string{"b", "c"}}
	out := Merge(parent, child)
	if !reflect.DeepEqual(out.PreWindow, []string{"a", "b", "c"}) {
		t.Errorf("PreWindow = %v", out.PreWindow)
	}
}

func TestMerge_TabsByTitle(t *testing.T) {
	parent := &spec.Project{Tabs: []spec.Tab{
		{Title: "claude", Cmd: "claude"},
		{Title: "dev", Panes: []spec.Pane{{Title: "server", Cmd: "overmind start"}}},
		{Title: "db", Cmd: "psql"},
	}}
	child := &spec.Project{Tabs: []spec.Tab{
		{Title: "db", Drop: true},
		{Title: "notes", Cmd: "nvim NOTES.md"},
		{Title: "dev", Panes: []spec.Pane{{Title: "logs", Cmd: "tail -f log/dev.log"}}},
	}}
	out := Merge(parent, child)
	if len(out.Tabs) != 3 {
		t.Fatalf("len(Tabs) = %d, want 3 (%+v)", len(out.Tabs), out.Tabs)
	}
	titles := []string{out.Tabs[0].Title, out.Tabs[1].Title, out.Tabs[2].Title}
	if !reflect.DeepEqual(titles, []string{"claude", "dev", "notes"}) {
		t.Errorf("titles = %v, want [claude dev notes]", titles)
	}
	devPanes := out.Tabs[1].Panes
	if len(devPanes) != 2 || devPanes[0].Title != "server" || devPanes[1].Title != "logs" {
		t.Errorf("dev panes = %+v", devPanes)
	}
}

func TestMerge_TabFieldsScalarChildWins(t *testing.T) {
	parent := &spec.Project{Tabs: []spec.Tab{{Title: "dev", Cmd: "old", Layout: "tiled"}}}
	child := &spec.Project{Tabs: []spec.Tab{{Title: "dev", Cmd: "new"}}}
	out := Merge(parent, child)
	if out.Tabs[0].Cmd != "new" {
		t.Errorf("cmd = %q, want new", out.Tabs[0].Cmd)
	}
	if out.Tabs[0].Layout != "tiled" {
		t.Errorf("layout = %q, want tiled (inherited)", out.Tabs[0].Layout)
	}
}

func TestMerge_DropOnNonExistentTitle_Ignored(t *testing.T) {
	parent := &spec.Project{Tabs: []spec.Tab{{Title: "a", Cmd: "x"}}}
	child := &spec.Project{Tabs: []spec.Tab{{Title: "ghost", Drop: true}}}
	out := Merge(parent, child)
	if len(out.Tabs) != 1 || out.Tabs[0].Title != "a" {
		t.Errorf("expected only [a], got %+v", out.Tabs)
	}
}

func TestMerge_PanesByTitle(t *testing.T) {
	parent := &spec.Project{Tabs: []spec.Tab{{Title: "dev", Panes: []spec.Pane{
		{Title: "p1", Cmd: "x"}, {Title: "p2", Cmd: "y"},
	}}}}
	child := &spec.Project{Tabs: []spec.Tab{{Title: "dev", Panes: []spec.Pane{
		{Title: "p2", Drop: true},
		{Title: "p3", Cmd: "z"},
	}}}}
	out := Merge(parent, child)
	titles := []string{}
	for _, pn := range out.Tabs[0].Panes {
		titles = append(titles, pn.Title)
	}
	if !reflect.DeepEqual(titles, []string{"p1", "p3"}) {
		t.Errorf("panes = %v, want [p1 p3]", titles)
	}
}
