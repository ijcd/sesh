package kitty

import (
	"context"
	"testing"
)

func TestCapture_NoOSWindows(t *testing.T) {
	fr := &fakeRunner{captureOut: `[]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	projects, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Errorf("expected empty slice, got %d projects", len(projects))
	}
}

func TestCapture_SingleTabSinglePane(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [
        {"title": "demo:shell", "windows": [
          {"is_focused": true, "cwd": "/tmp",
           "foreground_processes": [{"cmdline": ["zsh"]}]}
        ]}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	projects, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	p := projects[0]
	if p.Driver != "kitty" || p.Cwd != "/tmp" {
		t.Errorf("got %+v", p)
	}
	if len(p.Tabs) != 1 || p.Tabs[0].Title != "shell" {
		t.Errorf("expected one tab 'shell', got %+v", p.Tabs)
	}
	if p.Tabs[0].Cmd != "" {
		t.Errorf("zsh should normalize to empty, got %q", p.Tabs[0].Cmd)
	}
}

func TestCapture_MultiTabMultiPane(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [
        {"title": "demo:claude", "windows": [
          {"cwd": "/tmp", "foreground_processes": [{"cmdline": ["claude", "--continue"]}]}
        ]},
        {"title": "demo:dev", "windows": [
          {"cwd": "/tmp/x", "foreground_processes": [{"cmdline": ["overmind", "start"]}]},
          {"cwd": "/tmp/x", "foreground_processes": [{"cmdline": ["iex", "-S", "mix"]}]}
        ]}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	projects, _ := d.Capture(context.Background())
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	p := projects[0]
	if len(p.Tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %+v", p)
	}
	// Tab 0: single pane, Cmd populated
	if p.Tabs[0].Title != "claude" || p.Tabs[0].Cmd == "" {
		t.Errorf("tab 0 wrong: %+v", p.Tabs[0])
	}
	// Tab 1: multi-pane
	if p.Tabs[1].Title != "dev" || len(p.Tabs[1].Panes) != 2 {
		t.Errorf("tab 1 wrong: %+v", p.Tabs[1])
	}
	if p.Tabs[1].Panes[0].Title != "p1" || p.Tabs[1].Panes[1].Title != "p2" {
		t.Errorf("auto-titles wrong: %+v", p.Tabs[1].Panes)
	}
}

func TestCapture_OvermindNormalization(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [
        {"title": "demo:dev", "windows": [
          {"cwd": "/x",
           "foreground_processes": [{"cmdline": ["tmux", "-L", "overmind-abc", "attach"]}]}
        ]}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	projects, _ := d.Capture(context.Background())
	if len(projects) != 1 || projects[0].Tabs[0].Cmd != "overmind start" {
		t.Errorf("expected overmind normalization, got %+v", projects)
	}
}

func TestCapture_TwoOSWindows(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"id": 1, "is_focused": false, "tabs": [
        {"title": "alpha:shell", "windows": [{"cwd": "/alpha"}]}
      ]},
      {"id": 2, "is_focused": true, "tabs": [
        {"title": "beta:dev", "windows": [{"cwd": "/beta"}]}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	projects, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].Name != "alpha" {
		t.Errorf("first project name: got %q, want %q", projects[0].Name, "alpha")
	}
	if projects[1].Name != "beta" {
		t.Errorf("second project name: got %q, want %q", projects[1].Name, "beta")
	}
}

func TestCapture_NamingConsistentPrefix(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"id": 5, "is_focused": true, "tabs": [
        {"title": "project-x:shell", "windows": [{"cwd": "/x"}]},
        {"title": "project-x:dev",   "windows": [{"cwd": "/x"}]}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	projects, _ := d.Capture(context.Background())
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "project-x" {
		t.Errorf("got name %q, want %q", projects[0].Name, "project-x")
	}
}

func TestCapture_NamingNoPrefix(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"id": 7, "is_focused": true, "tabs": [
        {"title": "shell", "windows": [{"cwd": "/home"}]}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	projects, _ := d.Capture(context.Background())
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "kitty-os-7" {
		t.Errorf("got name %q, want %q", projects[0].Name, "kitty-os-7")
	}
}

func TestCapture_NamingMixedPrefixes(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"id": 3, "is_focused": true, "tabs": [
        {"title": "proj-a:shell", "windows": [{"cwd": "/a"}]},
        {"title": "proj-b:dev",   "windows": [{"cwd": "/b"}]}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	projects, _ := d.Capture(context.Background())
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "kitty-os-3" {
		t.Errorf("got name %q, want %q", projects[0].Name, "kitty-os-3")
	}
}
