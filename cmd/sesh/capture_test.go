package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/spec"
)

// TestNewCaptureCmd_LegacyName_EmitsYAML: positional name, single project from first driver.
func TestNewCaptureCmd_LegacyName_EmitsYAML(t *testing.T) {
	md := mock.New("tmux")
	md.CaptureProjects = []*spec.Project{
		{Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}}},
	}

	e := engine.New()
	e.Register(md)

	cmd := newCaptureCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"myproject"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "shell") {
		t.Errorf("expected YAML with tab 'shell', got %q", out)
	}
}

// TestNewCaptureCmd_List_PrintsSummary: no args → list mode with driver + project + tab count.
func TestNewCaptureCmd_List_PrintsSummary(t *testing.T) {
	tmuxDrv := mock.New("tmux")
	tmuxDrv.CaptureProjects = []*spec.Project{
		{Name: "my-project", Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}, {Title: "dev"}, {Title: "logs"}}},
		{Name: "another", Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}}},
	}
	kittyDrv := mock.New("kitty")
	kittyDrv.CaptureProjects = []*spec.Project{
		{Name: "liberties", Driver: "kitty", Tabs: []spec.Tab{{Title: "a"}, {Title: "b"}, {Title: "c"}, {Title: "d"}}},
	}

	e := engine.New()
	e.Register(tmuxDrv)
	e.Register(kittyDrv)

	cmd := newCaptureCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{}) // no args
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	checks := []string{
		"kitty:",
		"tmux:",
		"my-project (3 tabs)",
		"another (1 tab)",
		"liberties (4 tabs)",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in list output, got:\n%s", want, out)
		}
	}
}

// TestNewCaptureCmd_List_ExplicitFlag: --list flag same as no args.
func TestNewCaptureCmd_List_ExplicitFlag(t *testing.T) {
	md := mock.New("tmux")
	md.CaptureProjects = []*spec.Project{
		{Name: "proj", Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}}},
	}

	e := engine.New()
	e.Register(md)

	cmd := newCaptureCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "tmux:") {
		t.Errorf("expected 'tmux:' in list output, got %q", out)
	}
	if !strings.Contains(out, "proj (1 tab)") {
		t.Errorf("expected 'proj (1 tab)' in list output, got %q", out)
	}
}

// TestNewCaptureCmd_All_EmitsMultiDoc: --all emits multi-doc YAML with comments.
func TestNewCaptureCmd_All_EmitsMultiDoc(t *testing.T) {
	tmuxDrv := mock.New("tmux")
	tmuxDrv.CaptureProjects = []*spec.Project{
		{Name: "my-project", Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}}},
	}
	kittyDrv := mock.New("kitty")
	kittyDrv.CaptureProjects = []*spec.Project{
		{Name: "liberties", Driver: "kitty", Tabs: []spec.Tab{{Title: "editor"}}},
	}

	e := engine.New()
	e.Register(tmuxDrv)
	e.Register(kittyDrv)

	cmd := newCaptureCmd(e)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	checks := []string{
		"---",
		"# captured from kitty session: liberties",
		"# captured from tmux session: my-project",
		"shell",
		"editor",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in --all output, got:\n%s", want, out)
		}
	}
	// Two document separators.
	if count := strings.Count(out, "---"); count < 2 {
		t.Errorf("expected at least 2 '---' separators, got %d in:\n%s", count, out)
	}
}

// TestNewCaptureCmd_PositionalAll_Errors: 'capture all' is a common typo for '--all';
// reject with a hint instead of silently treating "all" as a project name.
func TestNewCaptureCmd_PositionalAll_Errors(t *testing.T) {
	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newCaptureCmd(e)
	cmd.SetArgs([]string{"all"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for positional 'all'")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Errorf("expected error to mention '--all', got: %v", err)
	}
}

// TestNewCaptureCmd_AllWithName_Errors: --all + positional name is mutually exclusive.
func TestNewCaptureCmd_AllWithName_Errors(t *testing.T) {
	e := engine.New()
	e.Register(mock.New("tmux"))

	cmd := newCaptureCmd(e)
	cmd.SetArgs([]string{"--all", "myname"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --all + positional name")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

// TestNewCaptureCmd_NoCaptures_EmptyOutput: drivers return empty slices.
func TestNewCaptureCmd_NoCaptures_EmptyOutput(t *testing.T) {
	t.Run("list mode prints no captures available", func(t *testing.T) {
		e := engine.New()
		e.Register(mock.New("tmux"))

		cmd := newCaptureCmd(e)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{}) // no args → list
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "no captures available") {
			t.Errorf("expected 'no captures available', got %q", out)
		}
	})

	t.Run("legacy mode errors", func(t *testing.T) {
		e := engine.New()
		e.Register(mock.New("tmux"))

		cmd := newCaptureCmd(e)
		cmd.SetArgs([]string{"myname"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error when no captures available")
		}
		if !strings.Contains(err.Error(), "no captures available") {
			t.Errorf("expected 'no captures available' in error, got: %v", err)
		}
	})

	t.Run("--all emits nothing", func(t *testing.T) {
		e := engine.New()
		e.Register(mock.New("tmux"))

		cmd := newCaptureCmd(e)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--all"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected empty output for --all with no captures, got %q", buf.String())
		}
	})
}
