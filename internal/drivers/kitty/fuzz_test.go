package kitty

import (
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func FuzzBuildCommands_NeverPanics(f *testing.F) {
	f.Add("demo", "/tmp", "shell", "")
	f.Add("demo", "/tmp", "claude", "claude --continue")
	f.Add("", "", "", "")
	f.Add("p", "/x", "t", string([]byte{0, 1, 2}))

	f.Fuzz(func(t *testing.T, name, cwd, tabTitle, tabCmd string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("BuildCommands panicked: %v", r)
			}
		}()
		p := &spec.Project{
			Name:   name,
			Driver: "kitty",
			Cwd:    cwd,
			Tabs:   []spec.Tab{{Title: tabTitle, Cmd: tabCmd}},
		}
		_, _ = BuildCommands(p)
	})
}

func FuzzSplitKittenCommand_NeverPanics(f *testing.F) {
	f.Add("kitten launch --type=tab --tab-title='demo:shell'")
	f.Add("kitten focus-tab --match tab_title:^demo$")
	f.Add("")
	f.Add("not-a-kitten-command")
	f.Add("kitten 'unterminated")

	f.Fuzz(func(t *testing.T, line string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("splitKittenCommand panicked: %v\ninput: %q", r, line)
			}
		}()
		_, _ = splitKittenCommand(line)
	})
}
