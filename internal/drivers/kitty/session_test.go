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
