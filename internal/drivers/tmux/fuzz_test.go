package tmux

import (
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

// FuzzBuildCommands_NeverPanics: BuildCommands must not panic on any input.
// We feed minimal seed corpus and let the fuzzer mutate.
func FuzzBuildCommands_NeverPanics(f *testing.F) {
	f.Add("demo", "tmux", "/tmp", "shell", "echo hi")
	f.Add("demo", "tmux", "/tmp", "dev", "")
	f.Add("", "", "", "", "")
	f.Add("p", "tmux", "/x", "t", string([]byte{0, 1, 2}))

	f.Fuzz(func(t *testing.T, name, driver, cwd, tabTitle, tabCmd string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("BuildCommands panicked: %v\ninput: name=%q driver=%q cwd=%q tab=%q cmd=%q",
					r, name, driver, cwd, tabTitle, tabCmd)
			}
		}()
		p := &spec.Project{
			Name:   name,
			Driver: driver,
			Cwd:    cwd,
			Tabs:   []spec.Tab{{Title: tabTitle, Cmd: tabCmd}},
		}
		_, _ = BuildCommands(p)
	})
}

// FuzzSplitTmuxCommand_NeverPanics: the parser must not panic on any input.
func FuzzSplitTmuxCommand_NeverPanics(f *testing.F) {
	f.Add("tmux new-session -d -s 'demo'")
	f.Add("tmux send-keys -t 'demo:dev.0' 'echo hi' Enter")
	f.Add("")
	f.Add("not-a-tmux-command")
	f.Add("tmux 'unterminated")
	f.Add(string([]byte{0, 0xff, 0xfe}))

	f.Fuzz(func(t *testing.T, line string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("splitTmuxCommand panicked: %v\ninput: %q", r, line)
			}
		}()
		_, _ = splitTmuxCommand(line)
	})
}
