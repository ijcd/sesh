package main

import (
	"fmt"

	"github.com/spf13/cobra"

	seshinit "github.com/ijcd/sesh/internal/init"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <shell>",
		Short: "Print shell init script for the discovery hook",
		Long: `Emit a shell snippet to eval from your rc:

    eval "$(sesh init zsh)"      # or bash, or fish

The hook prints "sesh: project here" when you cd into a directory
containing .sesh.yml. It never spawns anything.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := seshinit.Render(args[0])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), s)
			return nil
		},
	}
}
