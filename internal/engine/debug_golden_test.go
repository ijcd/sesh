package engine

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/spec"
	"github.com/ijcd/sesh/internal/testutil"
)

func TestDebug_Golden(t *testing.T) {
	cases := []struct {
		name string
		p    *spec.Project
	}{
		{
			name: "tmux-simple",
			p: &spec.Project{
				Name: "demo", Driver: "tmux", Cwd: "/tmp",
				Tabs: []spec.Tab{{Title: "shell"}},
			},
		},
		{
			name: "kitty-multi-tab",
			p: &spec.Project{
				Name: "demo", Driver: "kitty", Cwd: "/tmp",
				Tabs: []spec.Tab{
					{Title: "claude", Cmd: "claude --continue"},
					{Title: "shell"},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			md := mock.New(tc.p.Driver)
			md.DryRunVal = []string{"<mocked dry-run command for " + tc.name + ">"}
			e := New()
			e.Register(md)

			var buf bytes.Buffer
			if err := e.Debug(context.Background(), tc.p, false, &buf); err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", "golden", "debug-"+tc.name+".golden")
			testutil.Equal(t, buf.String(), golden)
		})
	}
}
