package main

import (
	"context"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/drivers/tmux"
	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/spec"
)

func newUpCmd(e *engine.Engine) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "up <name>",
		Short: "Launch a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Load(args[0], e.Drivers(), nil)
			if err != nil {
				return err
			}
			if err := e.Up(context.Background(), p, force); err != nil {
				return err
			}
			if p.Attach == nil || *p.Attach {
				return attachToTmux(p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Down + Up if a session already exists")
	return cmd
}

// attachToTmux replaces the current process with tmux attach-session (when
// outside tmux) or switch-client (when already inside tmux).
func attachToTmux(p *spec.Project) error {
	sessName := p.Session
	if sessName == "" {
		sessName = tmux.Slug(p.Name)
	}
	tmuxBin, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	var args []string
	if os.Getenv("TMUX") != "" {
		args = []string{"tmux", "switch-client", "-t", sessName}
	} else {
		args = []string{"tmux", "attach-session", "-t", sessName}
	}
	return syscall.Exec(tmuxBin, args, os.Environ())
}
