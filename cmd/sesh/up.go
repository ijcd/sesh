package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/engine"
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
			return e.Up(context.Background(), p, force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Down + Up if a session already exists")
	return cmd
}
