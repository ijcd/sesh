// Package launch spawns a fresh kitty instance bound to a sesh-controlled
// remote-control socket. Used by the cmd layer when sesh up --launch is
// invoked outside of an existing kitty.
package launch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// ErrTimeout is returned by WaitForSocket when the deadline expires before the
// socket file appears.
var ErrTimeout = errors.New("timed out waiting for kitty socket")

// SocketPathFor returns the canonical socket path for a project under stateDir,
// creating the sockets subdirectory if it doesn't exist.
func SocketPathFor(project, stateDir string) (string, error) {
	dir := filepath.Join(stateDir, "sockets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, project+".sock"), nil
}

// SpawnKitty starts a detached kitty process bound to socket. Returns the PID
// of the spawned process. The caller is responsible for waiting on the socket
// via WaitForSocket and persisting the PID/socket in state for later teardown.
func SpawnKitty(socket string) (int, error) {
	kitty, err := exec.LookPath("kitty")
	if err != nil {
		// macOS app-bundle fallbacks when kitty is not on PATH.
		for _, p := range []string{
			"/Applications/kitty.app/Contents/MacOS/kitty",
			"/opt/homebrew/bin/kitty",
		} {
			if _, e := os.Stat(p); e == nil {
				kitty = p
				err = nil
				break
			}
		}
		if err != nil {
			return 0, fmt.Errorf("kitty not found in PATH: %w", err)
		}
	}

	base := filepath.Base(socket)
	args := []string{
		"--listen-on=unix:" + socket,
		"--override", "allow_remote_control=yes",
		"--single-instance",
		"--instance-group=sesh-" + base,
	}
	cmd := exec.Command(kitty, args...)
	// Setpgid detaches the child into its own process group so it is not
	// killed when the sesh process exits.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Explicitly nil stdin/stdout/stderr — no file descriptors inherited.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("spawn kitty: %w", err)
	}
	// Don't Wait — the process must outlive sesh. It becomes a child of init
	// (pid 1) once sesh exits because Setpgid prevents reparenting to sesh.
	return cmd.Process.Pid, nil
}

// WaitForSocket polls every 20 ms until the socket file exists, timeout
// elapses (returning ErrTimeout), or ctx is cancelled.
func WaitForSocket(ctx context.Context, socket string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(socket); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %s", ErrTimeout, socket)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
