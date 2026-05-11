package tmux

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
	"github.com/ijcd/sesh/internal/testutil"
)

type goldenCase struct {
	name string
	p    *spec.Project
}

func goldenCases() []goldenCase {
	return []goldenCase{
		{
			name: "single-leaf-tab",
			p: &spec.Project{
				Name: "demo", Driver: "tmux", Cwd: "/tmp",
				Tabs: []spec.Tab{{Title: "shell"}},
			},
		},
		{
			name: "multi-tab-with-cmds",
			p: &spec.Project{
				Name: "demo", Driver: "tmux", Cwd: "/tmp",
				Tabs: []spec.Tab{
					{Title: "claude", Cmd: "claude --continue"},
					{Title: "dev", Cmd: "echo dev"},
				},
			},
		},
		{
			name: "multi-pane-with-layout",
			p: &spec.Project{
				Name: "demo", Driver: "tmux", Cwd: "/tmp",
				Tabs: []spec.Tab{{Title: "dev", Layout: "main-vertical",
					Panes: []spec.Pane{
						{Title: "server", Cmd: "overmind start"},
						{Title: "db", Cmd: "psql"},
					}}},
			},
		},
	}
}

func TestBuildCommands_Golden(t *testing.T) {
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			cmds, err := BuildCommands(tc.p)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(cmds, "\n") + "\n"
			golden := filepath.Join("testdata", "golden", tc.name+".golden")
			testutil.Equal(t, got, golden)
		})
	}
}
