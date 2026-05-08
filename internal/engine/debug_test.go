package engine

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/spec"
)

func TestDebug_PrintsSpecAndCommands(t *testing.T) {
	md := mock.New("tmux")
	md.DryRunVal = []string{"tmux new-session -d -s x", "tmux send-keys -t x.0 'echo' Enter"}
	e := New()
	e.Register(md)

	p := &spec.Project{
		Name: "x", Driver: "tmux",
		Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}},
	}

	var buf bytes.Buffer
	if err := e.Debug(context.Background(), p, false, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "driver: tmux") {
		t.Errorf("output missing spec dump: %s", out)
	}
	if !strings.Contains(out, "--- commands ---") {
		t.Errorf("output missing separator: %s", out)
	}
	if !strings.Contains(out, "tmux new-session") {
		t.Errorf("output missing dry-run: %s", out)
	}
}

func TestDebug_CommandsOnly(t *testing.T) {
	md := mock.New("tmux")
	md.DryRunVal = []string{"tmux new-session -d -s x"}
	e := New()
	e.Register(md)
	p := &spec.Project{Name: "x", Driver: "tmux", Tabs: []spec.Tab{{Title: "a", Cmd: "echo"}}}
	var buf bytes.Buffer
	if err := e.Debug(context.Background(), p, true, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "driver:") {
		t.Errorf("commands-only output should not include spec: %s", out)
	}
	if !strings.Contains(out, "tmux new-session") {
		t.Errorf("output missing commands: %s", out)
	}
}
