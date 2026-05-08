package tmux

import (
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func TestBuildCommands_SingleLeafTab(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "shell"}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) == 0 {
		t.Fatal("expected commands")
	}
	if !strings.Contains(cmds[0], "new-session") || !strings.Contains(cmds[0], "demo") {
		t.Errorf("first cmd should create session demo: %q", cmds[0])
	}
}

func TestBuildCommands_LeafTabWithCmd(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Cmd: "echo hi"}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmds, "\n")
	if !strings.Contains(joined, "send-keys") || !strings.Contains(joined, "echo hi") {
		t.Errorf("expected send-keys for cmd, got:\n%s", joined)
	}
}

func TestBuildCommands_MultipleTabsCreatesWindows(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{
			{Title: "a"}, {Title: "b"}, {Title: "c"},
		},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range cmds {
		if strings.Contains(c, "new-window") {
			n++
		}
	}
	if n != 2 { // first tab consumes the new-session window, then 2 new-window
		t.Errorf("expected 2 new-window cmds, got %d. cmds:\n%s", n, strings.Join(cmds, "\n"))
	}
}

func TestBuildCommands_PanesSplitAfterFirst(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Driver: "tmux",
			Panes: []spec.Pane{
				{Title: "p1", Cmd: "x"}, {Title: "p2", Cmd: "y"}, {Title: "p3", Cmd: "z"},
			}}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range cmds {
		if strings.Contains(c, "split-window") {
			n++
		}
	}
	if n != 2 {
		t.Errorf("expected 2 split-window cmds for 3 panes, got %d. cmds:\n%s", n, strings.Join(cmds, "\n"))
	}
}

func TestBuildCommands_LayoutApplied(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Driver: "tmux", Layout: "main-vertical",
			Panes: []spec.Pane{{Title: "p1", Cmd: "x"}, {Title: "p2", Cmd: "y"}}}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cmds {
		if strings.Contains(c, "select-layout") && strings.Contains(c, "main-vertical") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected select-layout command, got:\n%s", strings.Join(cmds, "\n"))
	}
}
