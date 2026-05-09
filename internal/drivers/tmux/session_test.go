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

func TestBuildCommands_PaneTargetFormat(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Driver: "tmux",
			Panes: []spec.Pane{
				{Title: "p1", Cmd: "a"},
				{Title: "p2", Cmd: "b"},
				{Title: "p3", Cmd: "c"},
			}}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmds, "\n")

	// split-window for panes 2 and 3 both target the window (not a pane index)
	splitCount := 0
	for _, c := range cmds {
		if strings.Contains(c, "split-window") && strings.Contains(c, "'demo:dev'") {
			splitCount++
		}
	}
	if splitCount != 2 {
		t.Errorf("expected 2 split-window targeting 'demo:dev', got %d\n%s", splitCount, joined)
	}

	// send-keys to second pane uses 'demo:dev.1'
	if !strings.Contains(joined, "'demo:dev.1'") {
		t.Errorf("expected send-keys to 'demo:dev.1'\n%s", joined)
	}
	// send-keys to third pane uses 'demo:dev.2'
	if !strings.Contains(joined, "'demo:dev.2'") {
		t.Errorf("expected send-keys to 'demo:dev.2'\n%s", joined)
	}
	// first pane: send-keys targets the window itself, no .N suffix
	if !strings.Contains(joined, "-t 'demo:dev' 'a' Enter") {
		t.Errorf("expected first pane send-keys to 'demo:dev' with no .N\n%s", joined)
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
