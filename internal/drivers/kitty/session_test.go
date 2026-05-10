package kitty

import (
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

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
	first := cmds[0]
	if !strings.Contains(first, "launch --type=tab") {
		t.Errorf("first cmd missing launch --type=tab: %s", first)
	}
	if !strings.Contains(first, "--tab-title='demo:shell'") {
		t.Errorf("first cmd missing tab title: %s", first)
	}
	if !strings.Contains(first, "--cwd='/tmp'") {
		t.Errorf("first cmd missing cwd: %s", first)
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
	first := cmds[0]
	if !strings.Contains(first, "--hold") {
		t.Errorf("expected --hold, got: %s", first)
	}
	if !strings.Contains(first, "-- /bin/sh -c 'claude --continue'") {
		t.Errorf("cmd should be sh -c wrapped: %s", first)
	}
}

func TestBuildCommands_CmdWithShellMetacharacters(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "x", Cmd: "echo \"hello world\" && tail -f log"}},
	}
	cmds, _ := BuildCommands(p)
	// The cmd must arrive intact inside the sh -c quoting; double-quotes
	// and && both must survive.
	if !strings.Contains(cmds[0], `'echo "hello world" && tail -f log'`) {
		t.Errorf("cmd not properly quoted: %s", cmds[0])
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
	// MatchTabTitle escapes ':' to '\:' in the regex; verify both focus-tab and the title appear.
	if !strings.Contains(last, "focus-tab") || !strings.Contains(last, `demo\:a`) {
		t.Errorf("last cmd should focus first tab 'a': %s", last)
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
	// MatchTabTitle escapes ':' to '\:' in the regex.
	if !strings.Contains(last, `demo\:b`) {
		t.Errorf("focus should target b: %s", last)
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
	joined := strings.Join(cmds, "\n")

	// Tab launched with first pane's title set
	if !strings.Contains(joined, `--type=tab`) {
		t.Errorf("missing tab launch: %s", joined)
	}
	// First pane: send command via --hold + sh -c wrap on tab launch (no separate split-window for pane 1)
	if !strings.Contains(joined, `-- /bin/sh -c 'x'`) {
		t.Errorf("first pane cmd should be on the tab launch with sh -c wrap: %s", joined)
	}
	// First pane window-title must be set after launch (match arg is quoted)
	if !strings.Contains(joined, `set-window-title --match 'tab_title:^demo\:dev$'`) {
		t.Errorf("first pane title not set: %s", joined)
	}

	// Second + third pane: split-window via launch --type=window
	splits := 0
	for _, c := range cmds {
		if strings.Contains(c, "launch --type=window") {
			splits++
		}
	}
	if splits != 2 {
		t.Errorf("expected 2 split-window cmds for 3 panes, got %d", splits)
	}

	// Layout default = splits when panes present
	foundLayout := false
	for _, c := range cmds {
		if strings.Contains(c, "goto-layout") && strings.Contains(c, "splits") {
			foundLayout = true
		}
	}
	if !foundLayout {
		t.Errorf("expected goto-layout splits: %s", joined)
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
	for _, c := range cmds {
		if strings.Contains(c, "goto-layout") && strings.Contains(c, "tall") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected goto-layout tall, got %s", strings.Join(cmds, "\n"))
	}
}

func TestBuildCommands_PanesUseLocationHsplit(t *testing.T) {
	p := &spec.Project{
		Name: "demo", Driver: "kitty", Cwd: "/tmp",
		Tabs: []spec.Tab{{Title: "dev", Driver: "kitty",
			Panes: []spec.Pane{{Title: "p1", Cmd: "x"}, {Title: "p2", Cmd: "y"}}}},
	}
	cmds, _ := BuildCommands(p)
	for _, c := range cmds {
		if strings.Contains(c, "launch --type=window") && !strings.Contains(c, "--location=hsplit") {
			t.Errorf("split-window cmd missing --location=hsplit: %s", c)
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
	for _, c := range cmds {
		if strings.Contains(c, "launch --type=window") {
			if !strings.Contains(c, "--cwd='/home/me/src/lib'") {
				t.Errorf("pane cwd should be joined: %s", c)
			}
		}
	}
}
