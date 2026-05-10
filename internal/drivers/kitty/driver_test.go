package kitty

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/spec"
)

func TestDriver_Name(t *testing.T) {
	d := New()
	if d.Name() != "kitty" {
		t.Errorf("Name = %q", d.Name())
	}
}

func TestDriver_Up_RunsBuiltCommands(t *testing.T) {
	fr := &fakeRunner{}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	p := &spec.Project{Name: "x", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "shell"}}}
	if err := d.Up(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if len(fr.runs) == 0 {
		t.Fatal("no kitten cmds run")
	}
	if !strings.Contains(fr.runs[0], "launch --type=tab") {
		t.Errorf("first run not launch --type=tab: %q", fr.runs[0])
	}
}

func TestDriver_Up_FailsWithoutKittyListenOn(t *testing.T) {
	fr := &fakeRunner{}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "")
	p := &spec.Project{Name: "x", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "shell"}}}
	err := d.Up(context.Background(), p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "KITTY_LISTEN_ON") {
		t.Errorf("err should mention KITTY_LISTEN_ON: %v", err)
	}
}

func TestDriver_Up_RunnerErrorAborts(t *testing.T) {
	fr := &fakeRunner{runErr: errors.New("boom")}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	p := &spec.Project{Name: "x", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "shell"}}}
	err := d.Up(context.Background(), p)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDriver_DryRun_NeedsNoSocket(t *testing.T) {
	d := New()
	t.Setenv("KITTY_LISTEN_ON", "")
	p := &spec.Project{Name: "x", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "shell"}}}
	cmds, err := d.DryRun(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) == 0 {
		t.Errorf("DryRun returned no commands")
	}
}

func TestDriver_Status_Exists(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [
        {"title": "demo:shell"}, {"title": "other:dev"}
      ]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	s, err := d.Status(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if s != drivers.StatusExists {
		t.Errorf("Status = %q, want exists", s)
	}
}

func TestDriver_Status_NotExists(t *testing.T) {
	fr := &fakeRunner{captureOut: `[
      {"is_focused": true, "tabs": [{"title": "other:thing"}]}
    ]`}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	s, err := d.Status(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if s != drivers.StatusNotExists {
		t.Errorf("Status = %q, want not_exists", s)
	}
}

func TestDriver_Status_RunnerError(t *testing.T) {
	fr := &fakeRunner{captureErr: errors.New("boom")}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	_, err := d.Status(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDriver_Down_ClosesProjectTabs(t *testing.T) {
	fr := &fakeRunner{}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	if err := d.Down(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if len(fr.runs) != 1 {
		t.Fatalf("expected 1 run, got %d: %v", len(fr.runs), fr.runs)
	}
	if !strings.Contains(fr.runs[0], "close-tab") {
		t.Errorf("expected close-tab: %s", fr.runs[0])
	}
	if !strings.Contains(fr.runs[0], "tab_title:^demo\\:.*$") {
		t.Errorf("expected prefix match for demo: %s", fr.runs[0])
	}
}

func TestDriver_Down_RunnerErrorSurfaced(t *testing.T) {
	fr := &fakeRunner{runErr: errors.New("nope")}
	d := newWith(fr)
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/sock")
	if err := d.Down(context.Background(), "demo"); err == nil {
		t.Fatal("expected error")
	}
}
