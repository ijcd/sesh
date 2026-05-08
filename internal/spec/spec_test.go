package spec

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestProject_UnmarshalYAML_Minimal(t *testing.T) {
	in := []byte(`
driver: tmux
cwd: /tmp/example
tabs:
  - title: shell
  - title: dev
    cmd: echo hi
`)
	var p Project
	if err := yaml.Unmarshal(in, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Driver != "tmux" {
		t.Errorf("driver = %q, want tmux", p.Driver)
	}
	if len(p.Tabs) != 2 {
		t.Fatalf("len(tabs) = %d, want 2", len(p.Tabs))
	}
	if p.Tabs[0].Title != "shell" {
		t.Errorf("tabs[0].title = %q", p.Tabs[0].Title)
	}
	if p.Tabs[1].Cmd != "echo hi" {
		t.Errorf("tabs[1].cmd = %q", p.Tabs[1].Cmd)
	}
}

func TestProject_UnmarshalYAML_FullShape(t *testing.T) {
	in := []byte(`
extends: phoenix
driver: tmux
cwd: /tmp/x
session: x
attach: false
startup_window: dev
startup_pane: server
vars:
  DB_NAME: x_dev
hooks:
  pre: ["direnv allow"]
  post: []
  on_project_start: ["echo started"]
  on_project_stop: ["echo stopped"]
pre_window:
  - "source .envrc"
tabs:
  - title: db
    drop: true
  - title: dev
    layout: main-vertical
    pre_window: ["pwd"]
    panes:
      - title: server
        cmd: overmind start
      - title: repl
        cmd: iex -S mix
`)
	var p Project
	if err := yaml.Unmarshal(in, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Extends != "phoenix" || *p.Attach != false || p.StartupPane != "server" {
		t.Errorf("top-level fields not parsed: %+v", p)
	}
	if p.Vars["DB_NAME"] != "x_dev" {
		t.Errorf("vars not parsed: %v", p.Vars)
	}
	if !p.Tabs[0].Drop {
		t.Errorf("drop:true on tabs[0] not parsed")
	}
	if p.Tabs[1].Layout != "main-vertical" || len(p.Tabs[1].Panes) != 2 {
		t.Errorf("dev tab not parsed: %+v", p.Tabs[1])
	}
	if p.Tabs[1].Panes[0].Title != "server" {
		t.Errorf("panes[0].title = %q", p.Tabs[1].Panes[0].Title)
	}
}
