package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/engine"
)

func newDebugCmd(e *engine.Engine) *cobra.Command {
	var commandsOnly bool
	cmd := &cobra.Command{
		Use:   "debug <name>",
		Short: "Print resolved spec + dry-run tmux commands",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Load(args[0], e.Drivers(), nil)
			if err != nil {
				return err
			}
			return e.Debug(context.Background(), p, commandsOnly, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&commandsOnly, "commands-only", false, "Print only the dry-run commands (no spec)")
	return cmd
}
