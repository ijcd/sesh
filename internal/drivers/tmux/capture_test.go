package tmux

import (
	"context"
	"testing"
)

func TestCapture_NoSession(t *testing.T) {
	fr := &fakeRunner{statusErr: nil, statusOut: ""}
	d := newWith(fr)
	p, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Errorf("expected nil project when no current session, got %+v", p)
	}
}

func TestCapture_ParsesWindowsAndPanes(t *testing.T) {
	fr := &fakeListRunner{
		currentSession: "demo",
		listWindows:    "claude\ndev\ndb\n",
		listPanesByWin: map[string]string{
			"claude": "claude --continue\n",
			"dev":    "overmind start\niex -S mix\n",
			"db":     "psql\n",
		},
	}
	d := newWith(fr)
	p, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected non-nil project")
	}
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

// fakeListRunner returns canned outputs based on argv.
type fakeListRunner struct {
	currentSession string
	listWindows    string
	listPanesByWin map[string]string
}

func (f *fakeListRunner) Run(ctx context.Context, args ...string) error { return nil }
func (f *fakeListRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	switch args[0] {
	case "display-message":
		return f.currentSession, nil
	case "list-windows":
		return f.listWindows, nil
	case "list-panes":
		// -t <session>:<window> -F '#{pane_current_command_full}'
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
