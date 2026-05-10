package kitty

import (
	"context"
	"testing"
)

func TestCapture_NoOSWindows(t *testing.T) {
	fr := &fakeRunner{captureOut: `[]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	p, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Errorf("expected nil project, got %+v", p)
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
	p, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected non-nil project")
	}
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
	p, _ := d.Capture(context.Background())
	if p == nil || len(p.Tabs) != 2 {
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
	p, _ := d.Capture(context.Background())
	if p.Tabs[0].Cmd != "overmind start" {
		t.Errorf("expected overmind normalization, got %q", p.Tabs[0].Cmd)
	}
}

func TestCapture_PrefersFocusedOSWindow(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"is_focused": false, "tabs": [{"title": "ignore:me"}]},
      {"is_focused": true, "tabs": [
        {"title": "demo:shell", "windows": [{"cwd": "/tmp"}]}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	p, _ := d.Capture(context.Background())
	if p == nil || p.Tabs[0].Title != "shell" {
		t.Errorf("expected focused-window's tab, got %+v", p)
	}
}
