// Package kitty is the kitty driver.
package kitty

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Runner is the seam for shelling out to kitten. Production uses execRunner
// (forks `kitten @ --to <socket> ...`); tests substitute a fake.
type Runner interface {
	Run(ctx context.Context, args ...string) error
	RunCapture(ctx context.Context, args ...string) (string, error)
}

type execRunner struct {
	kittenPath string
	socket     string // "" until KITTY_LISTEN_ON is read at Up time
}

func NewExecRunner() (*execRunner, error) {
	p, err := kittenPath()
	if err != nil {
		return nil, err
	}
	return &execRunner{kittenPath: p}, nil
}

// SetSocket assigns the kitten remote-control socket. Called by Driver
// methods just before issuing commands so KITTY_LISTEN_ON changes
// (e.g., from --launch) are picked up lazily.
func (r *execRunner) SetSocket(s string) { r.socket = s }

func (r *execRunner) fullArgs(args []string) []string {
	out := []string{"@"}
	if r.socket != "" {
		out = append(out, "--to", r.socket)
	}
	return append(out, args...)
}

func (r *execRunner) Run(ctx context.Context, args ...string) error {
	if r.socket == "" {
		return errors.New("kitty driver: KITTY_LISTEN_ON unset (run inside kitty or use --launch)")
	}
	cmd := exec.CommandContext(ctx, r.kittenPath, r.fullArgs(args)...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *execRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	if r.socket == "" {
		return "", errors.New("kitty driver: KITTY_LISTEN_ON unset (run inside kitty or use --launch)")
	}
	cmd := exec.CommandContext(ctx, r.kittenPath, r.fullArgs(args)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return out.String(), err
}

// kittenPath finds the kitten binary. Order: PATH, kitty +kitten,
// macOS app-bundle paths.
func kittenPath() (string, error) {
	if p, err := exec.LookPath("kitten"); err == nil {
		return p, nil
	}
	// Only check fallback paths if not in test mode (allows tests to
	// isolate the error path without needing to uninstall kitten).
	if os.Getenv("KITTEN_NO_FALLBACK") == "" {
		for _, p := range []string{
			"/Applications/kitty.app/Contents/MacOS/kitten",
			"/opt/homebrew/bin/kitten",
			"/usr/local/bin/kitten",
		} {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("kitten not found in PATH or known locations")
}
