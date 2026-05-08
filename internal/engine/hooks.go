package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// RunHooks runs each line via `sh -c`, in cwd. stdout/stderr stream to the
// caller's terminal. Non-zero exit aborts with hookName + the failed line.
func RunHooks(ctx context.Context, hookName string, lines []string, cwd string) error {
	for _, line := range lines {
		cmd := exec.CommandContext(ctx, "sh", "-c", line)
		cmd.Dir = cwd
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook %q: command %q failed: %w", hookName, line, err)
		}
	}
	return nil
}
