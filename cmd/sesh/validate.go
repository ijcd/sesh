package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/engine"
)

func newValidateCmd(e *engine.Engine) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <name>",
		Short: "Parse + merge + validate; do not spawn",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Load(args[0], e.Drivers(), nil)
			if err != nil {
				return err
			}
			if err := e.Validate(context.Background(), p); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ok: %s (%d tab(s), driver=%s)\n", p.Name, len(p.Tabs), p.Driver)
			return nil
		},
	}
}
