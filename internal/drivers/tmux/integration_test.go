//go:build integration

package tmux

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

func tmuxAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
}

func TestIntegration_UpThenDown(t *testing.T) {
	tmuxAvailable(t)
	socket := "sesh-test-up"
	name := "ittest"

	runTmux := func(args ...string) error {
		cmd := exec.Command("tmux", append([]string{"-L", socket}, args...)...)
		return cmd.Run()
	}

	// Cleanup any leftover.
	_ = runTmux("kill-session", "-t", name)

	d := New()
	// Replace runner so commands also target the test socket.
	d.r = socketRunner{socket: socket}

	p := &spec.Project{
		Name: name, Driver: "tmux", Cwd: "/tmp",
		Tabs: []spec.Tab{
			{Title: "a", Cmd: "true"},
			{Title: "b", Cmd: "true"},
		},
	}
	ctx := context.Background()

	if err := d.Up(ctx, p); err != nil {
		t.Fatalf("Up: %v", err)
	}
	defer runTmux("kill-session", "-t", name)

	out, err := exec.Command("tmux", "-L", socket, "list-windows", "-t", name, "-F", "#{window_name}").Output()
	if err != nil {
		t.Fatalf("list-windows: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if got != "a\nb" {
		t.Errorf("windows = %q, want a\\nb", got)
	}

	if err := d.Down(ctx, name); err != nil {
		t.Fatalf("Down: %v", err)
	}

	if err := runTmux("has-session", "-t", name); err == nil {
		t.Errorf("session %q should be gone", name)
	}
}

// socketRunner forces all tmux invocations to use a specific socket.
type socketRunner struct{ socket string }

func (s socketRunner) Run(ctx context.Context, args ...string) error {
	return exec.CommandContext(ctx, "tmux", append([]string{"-L", s.socket}, args...)...).Run()
}
func (s socketRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "tmux", append([]string{"-L", s.socket}, args...)...).Output()
	return string(out), err
}
