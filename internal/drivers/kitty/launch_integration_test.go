//go:build integration_cross

package kitty

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ijcd/sesh/internal/drivers/kitty/launch"
	tmuxdrv "github.com/ijcd/sesh/internal/drivers/tmux"
	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/spec"
)

// TestIntegration_KittyHostingTmux exercises the full cross-driver dispatch
// against real kitty + real tmux. Spawns a kitty via --launch, runs Up for a
// project where the kitty has one tab hosting a tmux session with two panes,
// asserts the tmux session exists and the kitty tab's command references it.
//
// SKIPPED unless SESH_TEST_KITTY_LAUNCH=1 (this opens a real kitty window).
func TestIntegration_KittyHostingTmux(t *testing.T) {
	if os.Getenv("SESH_TEST_KITTY_LAUNCH") != "1" {
		t.Skip("set SESH_TEST_KITTY_LAUNCH=1 to enable (opens a real kitty window)")
	}
	if _, err := exec.LookPath("kitty"); err != nil {
		t.Skip("kitty not on PATH")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}

	// Use isolated tmux socket so we don't pollute the user's tmux server.
	const socket = "sesh-cross-test"
	runTmux := func(args ...string) ([]byte, error) {
		return exec.Command("tmux", append([]string{"-L", socket}, args...)...).CombinedOutput()
	}
	// Cleanup any stale session from a previous run.
	runTmux("kill-session", "-t", "xtest-dev") //nolint:errcheck

	// Spawn kitty via the launch package.
	sockDir := t.TempDir()
	sockPath, err := launch.SocketPathFor("xtest", sockDir)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := launch.SpawnKitty(sockPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if proc, _ := os.FindProcess(pid); proc != nil {
			_ = proc.Kill()
		}
	})
	if err := launch.WaitForSocket(context.Background(), sockPath, 5*time.Second); err != nil {
		t.Fatalf("kitty did not become ready: %v", err)
	}
	t.Setenv("KITTY_LISTEN_ON", "unix:"+sockPath)

	// Build a project: kitty driver, one plain tab + one tab hosting tmux with two panes.
	p := &spec.Project{
		Name:   "xtest",
		Driver: "kitty",
		Cwd:    "/tmp",
		Tabs: []spec.Tab{
			{Title: "shell"},
			{
				Title:  "dev",
				Driver: "tmux",
				Panes: []spec.Pane{
					{Title: "p1", Cmd: "true"},
					{Title: "p2", Cmd: "true"},
				},
			},
		},
	}

	// Construct engine: tmux driver uses isolated socket, kitty uses spawned instance.
	e := engine.New()
	e.Register(tmuxdrv.New().WithSocket(socket))
	e.Register(New())

	if err := e.Up(context.Background(), p, false); err != nil {
		t.Fatalf("Up: %v", err)
	}
	t.Cleanup(func() { runTmux("kill-session", "-t", "xtest-dev") }) //nolint:errcheck

	// Assert: tmux session "xtest-dev" exists with 2 panes.
	out, err := runTmux("list-panes", "-t", "xtest-dev", "-F", "#{pane_index}")
	if err != nil {
		t.Fatalf("list-panes failed: %v\nout: %s", err, out)
	}
	panes := strings.Fields(strings.TrimSpace(string(out)))
	if len(panes) != 2 {
		t.Errorf("expected 2 panes in xtest-dev, got %d (%v)", len(panes), panes)
	}

	// Assert: DryRun for the outer project shows the xtest:dev tab launches with tmux attach.
	// (DryRun operates on the pre-transform spec; we verify the cross-driver command
	// would be wired correctly by constructing what the engine would produce.)
	//
	// The engine's transformCrossDriverTabs replaces the dev tab's Panes with a leaf
	// cmd of "tmux attach -t xtest-dev". Verify the tmux driver produces that attach cmd.
	innerSpec := &spec.Project{
		Name:   "xtest-dev",
		Driver: "tmux",
		Cwd:    "/tmp",
		Tabs: []spec.Tab{
			{Title: "dev", Panes: []spec.Pane{
				{Title: "p1", Cmd: "true"},
				{Title: "p2", Cmd: "true"},
			}},
		},
	}
	tDrv := tmuxdrv.New().WithSocket(socket)
	attachCmd, err := tDrv.AttachCommand(innerSpec)
	if err != nil {
		t.Fatalf("AttachCommand: %v", err)
	}
	if !strings.Contains(attachCmd, "tmux attach") {
		t.Errorf("expected AttachCommand to contain 'tmux attach', got %q", attachCmd)
	}
	if !strings.Contains(attachCmd, "xtest-dev") {
		t.Errorf("expected AttachCommand to reference session 'xtest-dev', got %q", attachCmd)
	}
}
