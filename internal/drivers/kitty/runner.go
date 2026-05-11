// Package kitty is the kitty driver.
package kitty

import (
	"context"
	"errors"
	"fmt"

	dexec "github.com/ijcd/sesh/internal/drivers/exec"
)

// Runner is the seam for shelling out to kitten. Production uses dexec.ExecRunner
// (forks `kitten @ --to <socket> ...`); tests substitute a fake.
type Runner = dexec.Runner

// newKittenRunner builds a Runner for kitten. When socket is non-empty the
// prefix is ["@", "--to", socket]; when empty the prefix is ["@"] and Run
// will return an error (the empty-socket guard is in kittenRun/kittenRunCapture
// wrappers used by the driver, not in the runner itself).
func newKittenRunner(kittenBin, socket string) Runner {
	prefix := []string{"@"}
	if socket != "" {
		prefix = append(prefix, "--to", socket)
	}
	return dexec.NewExecRunner(kittenBin, prefix)
}

// socketRequiredRunner wraps a Runner and rejects calls when socket is empty.
// This preserves the old behaviour: running without KITTY_LISTEN_ON returns a
// clear error rather than silently spawning a kitten process that will fail.
type socketRequiredRunner struct {
	inner  Runner
	socket string
}

func (r *socketRequiredRunner) Run(ctx context.Context, args ...string) error {
	if r.socket == "" {
		return errors.New("kitty driver: KITTY_LISTEN_ON unset (run inside kitty or use --launch)")
	}
	return r.inner.Run(ctx, args...)
}

func (r *socketRequiredRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	if r.socket == "" {
		return "", errors.New("kitty driver: KITTY_LISTEN_ON unset (run inside kitty or use --launch)")
	}
	return r.inner.RunCapture(ctx, args...)
}

// newSocketRequiredRunner wraps inner with an empty-socket guard.
func newSocketRequiredRunner(inner Runner, socket string) Runner {
	return &socketRequiredRunner{inner: inner, socket: socket}
}

// kittenPath finds the kitten binary. Order: PATH, macOS app-bundle paths.
// Set SESH_NO_FALLBACK=1 in tests to disable the fallback probes.
func kittenPath() (string, error) {
	p, err := dexec.FindBin(
		"kitten",
		"/Applications/kitty.app/Contents/MacOS/kitten",
		"/opt/homebrew/bin/kitten",
		"/usr/local/bin/kitten",
	)
	if err != nil {
		return "", fmt.Errorf("kitten not found in PATH or known locations")
	}
	return p, nil
}
