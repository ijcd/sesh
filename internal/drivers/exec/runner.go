// Package exec provides a generic exec-based Runner for driver implementations.
// Both the tmux and kitty drivers use this Runner to shell out to their
// respective binaries, prefixing per-driver arguments (e.g., "-L socket" for
// tmux, "@ --to socket" for kitty) before any caller-supplied args.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Runner is the seam for shelling out to an external binary. Production code
// uses NewExecRunner; tests substitute a fake.
type Runner interface {
	Run(ctx context.Context, args ...string) error
	RunCapture(ctx context.Context, args ...string) (string, error)
}

// ExecRunner executes binPath, prepending prefixArgs before every caller-supplied
// argument slice. Construct via NewExecRunner.
type ExecRunner struct {
	binPath    string
	prefixArgs []string
}

// NewExecRunner returns an ExecRunner that invokes binPath with prefixArgs
// prepended to every Run / RunCapture call.
func NewExecRunner(binPath string, prefixArgs []string) *ExecRunner {
	prefix := make([]string, len(prefixArgs))
	copy(prefix, prefixArgs)
	return &ExecRunner{binPath: binPath, prefixArgs: prefix}
}

func (r *ExecRunner) fullArgs(args []string) []string {
	out := make([]string, 0, len(r.prefixArgs)+len(args))
	out = append(out, r.prefixArgs...)
	out = append(out, args...)
	return out
}

// Run executes the command, discarding stdout. Stderr is forwarded to os.Stderr.
func (r *ExecRunner) Run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, r.binPath, r.fullArgs(args)...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunCapture executes the command, capturing and returning stdout.
// Stderr is forwarded to os.Stderr.
func (r *ExecRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.binPath, r.fullArgs(args)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return out.String(), err
}

// FindBin resolves the path for the first binary found among the given candidates.
// It checks PATH first, then falls back to the provided absolute fallback paths.
// The SESH_NO_FALLBACK environment variable (if set) disables the fallback probes,
// which is useful in tests.
//
// names must be non-empty. The first entry is the name looked up via PATH;
// subsequent entries are absolute fallback paths probed via os.Stat.
func FindBin(names ...string) (string, error) {
	if len(names) == 0 {
		return "", fmt.Errorf("exec.FindBin: no names provided")
	}
	// First element: PATH lookup by bare name.
	if p, err := exec.LookPath(names[0]); err == nil {
		return p, nil
	}
	if os.Getenv("SESH_NO_FALLBACK") == "" {
		for _, p := range names[1:] {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found in PATH or known locations", names[0])
}
