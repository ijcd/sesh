package main

import (
	"context"
	"os"
	"os/exec"
	"syscall"

	"github.com/ijcd/sesh/internal/drivers/tmux"
	"github.com/ijcd/sesh/internal/engine"
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

// runProject orchestrates launching a project: validate kitty requirements,
// spawn kitty if needed, run the engine, and attach to tmux if applicable.
// force and launchFlag control the --force and --launch CLI flags respectively.
func runProject(ctx context.Context, e *engine.Engine, p *spec.Project, force, launchFlag bool) error {
	if err := requiresKitty(p, launchFlag); err != nil {
		return err
	}
	if needsLaunch(p, launchFlag) {
		if err := performLaunch(ctx, p); err != nil {
			return err
		}
	}
	if err := e.Up(ctx, p, force); err != nil {
		return err
	}
	if p.Driver == "tmux" && (p.Attach == nil || *p.Attach) {
		return attachToTmux(p)
	}
	return nil
}
