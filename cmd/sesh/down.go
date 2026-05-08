package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/engine"
)

func newDownCmd(e *engine.Engine) *cobra.Command {
	return &cobra.Command{
		Use:   "down <name>",
		Short: "Stop a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Load(args[0], e.Drivers(), nil)
			if err != nil {
				return err
			}
			return e.Down(context.Background(), p)
		},
	}
}
