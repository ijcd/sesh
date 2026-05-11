package kitty

import (
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

// renderAll renders all argv slices to display strings, joined with newlines.
func renderAll(cmds [][]string) string { return strings.Join(RenderCommands(cmds), "\n") }

// containsArg reports whether any element of argv equals s.
func containsArg(argv []string, s string) bool {
	for _, a := range argv {
		if a == s {
			return true
		}
	}
	return false
}

// anyArgvContains reports whether any argv in cmds contains an element equal to s.
func anyArgvContains(cmds [][]string, s string) bool {
	for _, argv := range cmds {
		if containsArg(argv, s) {
			return true
		}
	}
	return false
}

// anyArgvHasSubcmd reports whether any argv starts with subCmd.
func anyArgvHasSubcmd(cmds [][]string, subCmd string) bool {
	for _, argv := range cmds {
		if len(argv) > 0 && argv[0] == subCmd {
			return true
		}
	}
	return false
}

// firstArgv returns the first argv slice, or nil if cmds is empty.
func firstArgv(cmds [][]string) []string {
	if len(cmds) == 0 {
		return nil
	}
	return cmds[0]
}

func TestBuildCommands_LeafTabNoCmd(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "shell"}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) < 1 {
		t.Fatal("expected at least 1 cmd")
	}
	first := firstArgv(cmds)
	if first[0] != "launch" {
		t.Errorf("first cmd should be launch: %v", first)
	}
	if !containsArg(first, "--type=tab") {
		t.Errorf("first cmd missing --type=tab: %v", first)
	}
	if !containsArg(first, "--tab-title=demo:shell") {
		t.Errorf("first cmd missing --tab-title=demo:shell: %v", first)
	}
	if !containsArg(first, "--cwd=/tmp") {
		t.Errorf("first cmd missing --cwd=/tmp: %v", first)
	}
}

func TestBuildCommands_LeafTabWithCmd_UsesHoldAndShellWrap(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "claude", Cmd: "claude --continue"}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	first := firstArgv(cmds)
	if !containsArg(first, "--hold") {
		t.Errorf("expected --hold, got: %v", first)
	}
	if !containsArg(first, "-c") || !containsArg(first, "claude --continue") {
		t.Errorf("cmd should include -c and the command: %v", first)
	}
}

func TestBuildCommands_CmdWithShellMetacharacters(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "x", Cmd: `echo "hello world" && tail -f log`}},
	}
	cmds, _ := BuildCommands(p)
	first := firstArgv(cmds)
	// The cmd must arrive intact as a single argv element (no splitting).
	if !containsArg(first, `echo "hello world" && tail -f log`) {
		t.Errorf("cmd not intact as single argv element: %v", first)
	}
}

func TestBuildCommands_FocusFirstTabByDefault(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{
			{Title: "a"}, {Title: "b"},
		},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	last := cmds[len(cmds)-1]
	// focus-tab targeting demo:a (MatchTabTitle escapes colon to \: in regex)
	if last[0] != "focus-tab" {
		t.Errorf("last cmd should be focus-tab: %v", last)
	}
	if !containsArg(last, MatchTabTitle(ProjectTabTitle("demo", "a"))) {
		t.Errorf("last cmd should target demo:a: %v", last)
	}
}

func TestBuildCommands_StartupWindowOverridesFocus(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		StartupWindow: "b",
		Tabs: []spec.Tab{
			{Title: "a"}, {Title: "b"},
		},
	}
	cmds, _ := BuildCommands(p)
	last := cmds[len(cmds)-1]
	if !containsArg(last, MatchTabTitle(ProjectTabTitle("demo", "b"))) {
		t.Errorf("focus should target b: %v", last)
	}
}

func TestBuildCommands_ErrorOnNoTabs(t *testing.T) {
	p := &spec.Project{Name: "demo", Driver: "kitty", Cwd: "/tmp"}
	_, err := BuildCommands(p)
	if err == nil {
		t.Fatal("expected error on no tabs")
	}
}

func TestBuildCommands_MultiPaneTab(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Driver: "kitty",
			Panes: []spec.Pane{
				{Title: "p1", Cmd: "x"},
				{Title: "p2", Cmd: "y"},
				{Title: "p3", Cmd: "z"},
			}}},
	}
	cmds, err := BuildCommands(p)
	if err != nil {
		t.Fatal(err)
	}

	// First argv: launch --type=tab
	first := firstArgv(cmds)
	if !containsArg(first, "--type=tab") {
		t.Errorf("missing --type=tab launch: %v", first)
	}
	// First pane cmd via sh -c
	if !containsArg(first, "x") || !containsArg(first, "-c") {
		t.Errorf("first pane cmd should be on tab launch with sh -c: %v", first)
	}

	// First pane window-title must be set after launch
	found := false
	for _, argv := range cmds {
		if argv[0] == "set-window-title" {
			found = true
		}
	}
	if !found {
		t.Errorf("first pane title not set:\n%s", renderAll(cmds))
	}

	// Second + third pane: launch --type=window
	splits := 0
	for _, argv := range cmds {
		if argv[0] == "launch" && containsArg(argv, "--type=window") {
			splits++
		}
	}
	if splits != 2 {
		t.Errorf("expected 2 split-window cmds for 3 panes, got %d", splits)
	}

	// Layout default = splits when panes present
	foundLayout := false
	for _, argv := range cmds {
		if argv[0] == "goto-layout" && containsArg(argv, "splits") {
			foundLayout = true
		}
	}
	if !foundLayout {
		t.Errorf("expected goto-layout splits:\n%s", renderAll(cmds))
	}
}

func TestBuildCommands_MultiPaneTabRespectsLayout(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Driver: "kitty", Layout: "tall",
			Panes: []spec.Pane{{Title: "p1", Cmd: "x"}, {Title: "p2", Cmd: "y"}}}},
	}
	cmds, _ := BuildCommands(p)
	found := false
	for _, argv := range cmds {
		if argv[0] == "goto-layout" && containsArg(argv, "tall") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected goto-layout tall:\n%s", renderAll(cmds))
	}
}

func TestBuildCommands_PanesUseLocationHsplit(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Driver: "kitty",
			Panes: []spec.Pane{{Title: "p1", Cmd: "x"}, {Title: "p2", Cmd: "y"}}}},
	}
	cmds, _ := BuildCommands(p)
	for _, argv := range cmds {
		if argv[0] == "launch" && containsArg(argv, "--type=window") {
			if !containsArg(argv, "--location=hsplit") {
				t.Errorf("split-window cmd missing --location=hsplit: %v", argv)
			}
		}
	}
}

func TestBuildCommands_PaneCwdRelativeToTabCwd(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/home/me",
		Tabs: []spec.Tab{{Title: "dev", Driver: "kitty", Cwd: "src",
			Panes: []spec.Pane{
				{Title: "p1", Cmd: "x", Cwd: "lib"},
			}}},
	}
	cmds, _ := BuildCommands(p)
	for _, argv := range cmds {
		if argv[0] == "launch" && containsArg(argv, "--type=window") {
			if !containsArg(argv, "--cwd=/home/me/src/lib") {
				t.Errorf("pane cwd should be joined: %v", argv)
			}
		}
	}
}
