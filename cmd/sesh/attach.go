package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/ijcd/sesh/internal/drivers/tmux"
	"github.com/ijcd/sesh/internal/spec"
)

// attachArgs returns the argv for the tmux binary to attach/switch to p's
// session. inTmux should reflect whether $TMUX is set.
func attachArgs(p *spec.Project, inTmux bool) []string {
	sess := p.Session
	if sess == "" {
		sess = tmux.Slug(p.Name)
	}
	if inTmux {
		return []string{"tmux", "switch-client", "-t", sess}
	}
	return []string{"tmux", "attach-session", "-t", sess}
}

// attachToTmux replaces the current process with tmux attach-session (when
// outside tmux) or switch-client (when already inside tmux).
func attachToTmux(p *spec.Project) error {
	tmuxBin, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	args := attachArgs(p, os.Getenv("TMUX") != "")
	return syscall.Exec(tmuxBin, args, os.Environ())
}
