package tmux

import (
	"context"
	"errors"
	"testing"
)

func TestCapture_NoSession(t *testing.T) {
	// list-sessions returns error → no tmux server running → empty slice, no error.
	fr := &fakeRunner{statusErr: errors.New("no server running"), statusOut: ""}
	d := newWith(fr)
	projects, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Errorf("expected empty slice when no session, got %d projects", len(projects))
	}
}

func TestCapture_ParsesWindowsAndPanes(t *testing.T) {
	fr := &fakeListRunner{
		sessions:    "demo\n",
		listWindows: "claude\ndev\ndb\n",
		listPanesByWin: map[string]string{
			"claude": "claude --continue\n",
			"dev":    "overmind start\niex -S mix\n",
			"db":     "psql\n",
		},
	}
	d := newWith(fr)
	projects, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	p := projects[0]
	if p.Name != "demo" || p.Driver != "tmux" {
		t.Errorf("project = %+v", p)
	}
	if len(p.Tabs) != 3 {
		t.Fatalf("len(Tabs) = %d, want 3", len(p.Tabs))
	}
	if p.Tabs[1].Title != "dev" || len(p.Tabs[1].Panes) != 2 {
		t.Errorf("dev tab wrong: %+v", p.Tabs[1])
	}
	// Single-pane window should fold into Tab.Cmd, not Tab.Panes.
	if p.Tabs[0].Cmd == "" {
		t.Errorf("single-pane should set Tab.Cmd, got %+v", p.Tabs[0])
	}
}

func TestCapture_MultipleSessionsReturnsMultipleProjects(t *testing.T) {
	fr := &fakeListRunner{
		sessions:       "alpha\nbeta\n",
		listWindows:    "shell\n",
		listPanesByWin: map[string]string{"shell": "zsh\n"},
	}
	d := newWith(fr)
	projects, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects for 2 sessions, got %d", len(projects))
	}
	names := map[string]bool{projects[0].Name: true, projects[1].Name: true}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("unexpected project names: %v, %v", projects[0].Name, projects[1].Name)
	}
}

// fakeListRunner returns canned outputs based on argv.
type fakeListRunner struct {
	sessions       string
	listWindows    string
	listPanesByWin map[string]string
}

func (f *fakeListRunner) Run(ctx context.Context, args ...string) error { return nil }
func (f *fakeListRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	switch args[0] {
	case "list-sessions":
		return f.sessions, nil
	case "list-windows":
		return f.listWindows, nil
	case "list-panes":
		// -t <session>:<window> -F '#{pane_current_command}'
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "-t" {
				target := args[i+1]
				// target = "session:window"
				colon := -1
				for j, c := range target {
					if c == ':' {
						colon = j
						break
					}
				}
				if colon >= 0 {
					return f.listPanesByWin[target[colon+1:]], nil
				}
			}
		}
	}
	return "", nil
}
