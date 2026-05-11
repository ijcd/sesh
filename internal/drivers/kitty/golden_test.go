package kitty

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
	"github.com/ijcd/sesh/internal/testutil"
)

func TestBuildCommands_Golden(t *testing.T) {
	cases := []struct {
		name string
		p    *spec.Project
	}{
		{
			name: "single-leaf-tab",
			p: &spec.Project{
				Name: "demo", Driver: "kitty", Cwd: "/tmp",
				Tabs: []spec.Tab{{Title: "shell"}},
			},
		},
		{
			name: "leaf-tab-with-cmd-shell-wrapped",
			p: &spec.Project{
				Name: "demo", Driver: "kitty", Cwd: "/tmp",
				Tabs: []spec.Tab{{Title: "claude", Cmd: "claude --continue"}},
			},
		},
		{
			name: "multi-pane-splits",
			p: &spec.Project{
				Name: "demo", Driver: "kitty", Cwd: "/tmp",
				Tabs: []spec.Tab{{Title: "dev", Driver: "kitty",
					Panes: []spec.Pane{
						{Title: "p1", Cmd: "x"},
						{Title: "p2", Cmd: "y"},
					}}},
			},
		},
	}

	for _, tc := range cases {
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
