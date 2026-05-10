package engine

import (
	"context"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/spec"
)

func TestUp_CrossDriverDispatch(t *testing.T) {
	tmux := mock.New("tmux")
	tmux.AttachCommandVal = "tmux attach -t demo-dev"
	kitty := mock.New("kitty")
	e := New()
	e.Register(tmux)
	e.Register(kitty)

	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{
			{Title: "claude", Cmd: "claude --continue"},
			{Title: "dev", Driver: "tmux", Panes: []spec.Pane{
				{Title: "server", Cmd: "overmind start"},
				{Title: "repl", Cmd: "iex -S mix"},
			}},
		},
	}

	if err := e.Up(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}

	// tmux driver got an inner project for the dev tab
	if len(tmux.UpCalls) != 1 {
		t.Fatalf("expected tmux.Up called once, got %d", len(tmux.UpCalls))
	}
	inner := tmux.UpCalls[0]
	if inner.Driver != "tmux" || len(inner.Tabs) != 1 || inner.Tabs[0].Title != "dev" {
		t.Errorf("inner project wrong: %+v", inner)
	}

	// kitty driver got the outer project, but tab 'dev' should now be a leaf
	// with cmd = "tmux attach -t demo-dev" and no Panes.
	if len(kitty.UpCalls) != 1 {
		t.Fatalf("expected kitty.Up called once, got %d", len(kitty.UpCalls))
	}
	outer := kitty.UpCalls[0]
	var devTab *spec.Tab
	for i := range outer.Tabs {
		if outer.Tabs[i].Title == "dev" {
			devTab = &outer.Tabs[i]
		}
	}
	if devTab == nil {
		t.Fatal("dev tab missing from outer")
	}
	if devTab.Cmd != "tmux attach -t demo-dev" {
		t.Errorf("dev.Cmd = %q, want attach string", devTab.Cmd)
	}
	if len(devTab.Panes) != 0 {
		t.Errorf("dev.Panes should be empty, got %v", devTab.Panes)
	}
	if devTab.Driver != "" {
		t.Errorf("dev.Driver should be cleared (now leaf), got %q", devTab.Driver)
	}
}
