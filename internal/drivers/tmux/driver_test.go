package tmux

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/spec"
)

type fakeRunner struct {
	runs      []string
	runErr    error
	statusOut string
	statusErr error
}

func (f *fakeRunner) Run(ctx context.Context, args ...string) error {
	f.runs = append(f.runs, "tmux "+strings.Join(args, " "))
	return f.runErr
}
func (f *fakeRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	return f.statusOut, f.statusErr
}

func TestDriver_Name(t *testing.T) {
	d := New()
	if d.Name() != "tmux" {
		t.Errorf("Name = %q, want tmux", d.Name())
	}
}

func TestDriver_Up_RunsBuiltCommands(t *testing.T) {
	fr := &fakeRunner{}
	d := newWith(fr)
	p := &spec.Project{Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "a"}}}
	if err := d.Up(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if len(fr.runs) == 0 {
		t.Fatal("no tmux commands run")
	}
	if !strings.Contains(fr.runs[0], "new-session") {
		t.Errorf("first run should be new-session: %q", fr.runs[0])
	}
}

func TestDriver_Down_KillsSession(t *testing.T) {
	fr := &fakeRunner{}
	d := newWith(fr)
	if err := d.Down(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if len(fr.runs) != 1 || !strings.Contains(fr.runs[0], "kill-session") {
		t.Errorf("expected kill-session, got %v", fr.runs)
	}
}

func TestDriver_Status_Exists(t *testing.T) {
	fr := &fakeRunner{statusOut: ""} // has-session exits 0 with empty stdout
	d := newWith(fr)
	s, err := d.Status(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if s != drivers.StatusExists {
		t.Errorf("Status = %q, want exists", s)
	}
}

func TestDriver_Status_NotExists(t *testing.T) {
	fr := &fakeRunner{statusErr: errors.New("exit status 1")}
	d := newWith(fr)
	s, err := d.Status(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if s != drivers.StatusNotExists {
		t.Errorf("Status = %q, want not_exists", s)
	}
}

func TestDriver_DryRun_ReturnsSameAsBuildCommands(t *testing.T) {
	d := New()
	p := &spec.Project{Name: "x", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "a"}}}
	cmds, err := d.DryRun(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) == 0 {
		t.Errorf("DryRun returned no commands")
	}
}
