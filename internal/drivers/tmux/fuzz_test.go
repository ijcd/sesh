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
		cmds, _ := BuildCommands(p)
		_ = RenderCommands(cmds)
	})
}
