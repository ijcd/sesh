package tmux

import (
	"context"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

// joinArgv joins an argv slice into a space-separated string for assertion messages.
func joinArgv(argv []string) string { return strings.Join(argv, " ") }

// renderAll renders all argv slices to display strings, joined with newlines.
func renderAll(cmds [][]string) string { return strings.Join(RenderCommands(cmds), "\n") }

// captureRunner returns a fixed string for RunCapture calls keyed by joined args.
type captureRunner struct {
	outputs map[string]string
}

func (r *captureRunner) Run(_ context.Context, _ ...string) error { return nil }
func (r *captureRunner) RunCapture(_ context.Context, args ...string) (string, error) {
	return r.outputs[strings.Join(args, " ")], nil
}

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
	first := cmds[0]
	if first[0] != "new-session" {
		t.Errorf("first argv[0] should be new-session: %q", joinArgv(first))
	}
	found := false
	for _, a := range first {
		if a == "demo" {
			found = true
		}
	}
	if !found {
		t.Errorf("first cmd should reference session demo: %q", joinArgv(first))
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
	rendered := renderAll(cmds)
	if !strings.Contains(rendered, "send-keys") || !strings.Contains(rendered, "echo hi") {
		t.Errorf("expected send-keys for cmd, got:\n%s", rendered)
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
	for _, argv := range cmds {
		if argv[0] == "new-window" {
			n++
		}
	}
	if n != 2 { // first tab consumes the new-session window, then 2 new-window
		t.Errorf("expected 2 new-window cmds, got %d. cmds:\n%s", n, renderAll(cmds))
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
	for _, argv := range cmds {
		if argv[0] == "split-window" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("expected 2 split-window cmds for 3 panes, got %d. cmds:\n%s", n, renderAll(cmds))
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

	// split-window for panes 2 and 3 both target the window (not a pane index)
	splitCount := 0
	for _, argv := range cmds {
		if argv[0] == "split-window" && containsArg(argv, "demo:dev") {
			splitCount++
		}
	}
	if splitCount != 2 {
		t.Errorf("expected 2 split-window targeting demo:dev, got %d\n%s", splitCount, renderAll(cmds))
	}

	// send-keys to second pane uses target demo:dev.1
	if !anyArgvContainsArg(cmds, "demo:dev.1") {
		t.Errorf("expected argv with demo:dev.1\n%s", renderAll(cmds))
	}
	// send-keys to third pane uses target demo:dev.2
	if !anyArgvContainsArg(cmds, "demo:dev.2") {
		t.Errorf("expected argv with demo:dev.2\n%s", renderAll(cmds))
	}
	// first pane: send-keys targets the window itself (demo:dev), with cmd "a"
	found := false
	for _, argv := range cmds {
		if argv[0] == "send-keys" && containsArg(argv, "demo:dev") && containsArg(argv, "a") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected send-keys to demo:dev with cmd a\n%s", renderAll(cmds))
	}
}

func containsArg(argv []string, s string) bool {
	for _, a := range argv {
		if a == s {
			return true
		}
	}
	return false
}

func anyArgvContainsArg(cmds [][]string, s string) bool {
	for _, argv := range cmds {
		if containsArg(argv, s) {
			return true
		}
	}
	return false
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
	for _, argv := range cmds {
		if argv[0] == "select-layout" && containsArg(argv, "main-vertical") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected select-layout command, got:\n%s", renderAll(cmds))
	}
}

func TestBuildCommands_RespectsPaneBaseIndex(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Driver: "tmux",
			Panes: []spec.Pane{
				{Title: "p1", Cmd: "a"},
				{Title: "p2", Cmd: "b"},
				{Title: "p3", Cmd: "c"},
			}}},
	}
	cmds, err := BuildCommandsWithOpts(p, BuildOpts{PaneBaseIndex: 1})
	if err != nil {
		t.Fatal(err)
	}

	// With pane-base-index=1: send-keys to second pane uses .2, third uses .3.
	// (First pane's select-pane still uses .1, that's expected.)
	sendKeysTargets := []string{}
	for _, argv := range cmds {
		if argv[0] == "send-keys" && len(argv) >= 3 {
			sendKeysTargets = append(sendKeysTargets, argv[2]) // argv[2] is the -t value
		}
	}
	found2, found3, found1 := false, false, false
	for _, tgt := range sendKeysTargets {
		switch tgt {
		case "demo:dev.2":
			found2 = true
		case "demo:dev.3":
			found3 = true
		case "demo:dev.1":
			found1 = true
		}
	}
	if !found2 {
		t.Errorf("expected send-keys to demo:dev.2 with paneBaseIndex=1\n%s", renderAll(cmds))
	}
	if !found3 {
		t.Errorf("expected send-keys to demo:dev.3 with paneBaseIndex=1\n%s", renderAll(cmds))
	}
	if found1 {
		t.Errorf("unexpected send-keys to demo:dev.1 with paneBaseIndex=1\n%s", renderAll(cmds))
	}
}

func TestBuildCommands_TabCwdAbsolute(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/home/me",
		Tabs: []spec.Tab{{Title: "t", Cwd: "/etc"}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	if !anyArgvContainsArg(cmds, "/etc") {
		t.Errorf("expected tab cwd /etc in cmds:\n%s", renderAll(cmds))
	}
	if anyArgvContainsArg(cmds, "/home/me") {
		t.Errorf("should not contain project cwd /home/me when tab cwd is absolute:\n%s", renderAll(cmds))
	}
}

func TestBuildCommands_TabCwdRelative(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/home/me",
		Tabs: []spec.Tab{{Title: "t", Cwd: "src"}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	if !anyArgvContainsArg(cmds, "/home/me/src") {
		t.Errorf("expected joined cwd /home/me/src in cmds:\n%s", renderAll(cmds))
	}
}

func TestBuildCommands_TabCwdInherits(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/home/me",
		Tabs: []spec.Tab{{Title: "t"}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	if !anyArgvContainsArg(cmds, "/home/me") {
		t.Errorf("expected project cwd /home/me inherited:\n%s", renderAll(cmds))
	}
}

func TestBuildCommands_PaneCwdRelativeToTabCwd(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "tmux", Cwd: "/home/me",
		Tabs: []spec.Tab{{Title: "t", Cwd: "src",
			Panes: []spec.Pane{
				{Title: "p1", Cmd: "a"},
				{Title: "p2", Cmd: "b", Cwd: "lib"},
			}}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	// pane cwd "lib" should be joined to tab cwd "/home/me/src" → "/home/me/src/lib"
	if !anyArgvContainsArg(cmds, "/home/me/src/lib") {
		t.Errorf("expected pane cwd /home/me/src/lib in cmds:\n%s", renderAll(cmds))
	}
}

func TestQueryIntOption(t *testing.T) {
	tests := []struct {
		output string
		want   int
	}{
		{"1\n", 1},
		{"0\n", 0},
		{"", 42},          // empty → default
		{"garbage\n", 42}, // parse error → default
	}
	ctx := context.Background()
	for _, tc := range tests {
		r := &captureRunner{outputs: map[string]string{"show-options -gv pane-base-index": tc.output}}
		got := queryIntOption(ctx, r, "pane-base-index", 42)
		if got != tc.want {
			t.Errorf("output=%q: got %d, want %d", tc.output, got, tc.want)
		}
	}
}
