package engine

import (
	"context"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func TestDown_RunsOnStopThenDown(t *testing.T) {
	e, md := newTestEngine()
	p := &spec.Project{
		Name: "x", Driver: "tmux", Cwd: "/tmp",
		Hooks: spec.Hooks{OnStop: []string{"echo bye"}},
	}
	if err := e.Down(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if len(md.DownCalls) != 1 || md.DownCalls[0] != "x" {
		t.Errorf("Down call wrong: %v", md.DownCalls)
	}
}
