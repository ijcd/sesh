package main

import (
	"fmt"

	"github.com/spf13/cobra"

	seshinit "github.com/ijcd/sesh/internal/init"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <target>",
		Short: "Print init snippet for a shell hook or the emacs recipe",
		Long: `Emit an init snippet for the given target.

Shells (zsh / bash / fish) -- eval from your rc:

    eval "$(sesh init zsh)"

The shell hook prints "sesh: project here" when you cd into a directory
containing .sesh.yml. It never spawns anything.

Emacs -- load from init.el:

    sesh init emacs > ~/.emacs.d/sesh.el
    ;; (load-file "~/.emacs.d/sesh.el")

Emits the default sesh-open-project / sesh-close-project hooks (tab-bar
based). Replace with your own policy at will -- sesh only dispatches.`,
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
