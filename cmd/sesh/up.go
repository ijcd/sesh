package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/spec"
)

func newUpCmd(e *engine.Engine) *cobra.Command {
	var force bool
	var launchFlag bool
	cmd := &cobra.Command{
		Use:   "up [name]",
		Short: "Launch a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			switch len(args) {
			case 1:
				name = args[0]
			case 0:
				name = os.Getenv("SESH_PROJECT")
				if name == "" {
					return fmt.Errorf("sesh up requires a project name or $SESH_PROJECT env var (set by direnv or shell)")
				}
			}
			p, err := config.Load(name, e.Drivers(), nil)
			if err != nil {
				return err
			}
			if err := requiresKitty(p, launchFlag); err != nil {
				return err
			}
			ctx := context.Background()
			if needsLaunch(p, launchFlag) {
				if err := performLaunch(ctx, p); err != nil {
					return err
				}
			}
			if err := e.Up(ctx, p, force); err != nil {
				return err
			}
			if p.Attach == nil || *p.Attach {
				return attachIfTmux(p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Down + Up if a session already exists")
	cmd.Flags().BoolVar(&launchFlag, "launch", false, "Spawn a new kitty if not already inside one (kitty driver only)")
	return cmd
}

func attachIfTmux(p *spec.Project) error {
	if p.Driver != "tmux" {
		return nil
	}
	return attachToTmux(p)
}
