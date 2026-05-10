package main

import (
	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/drivers/kitty"
	"github.com/ijcd/sesh/internal/drivers/tmux"
	"github.com/ijcd/sesh/internal/engine"
)

const Version = "0.1.0-dev"

func newRootCmd() *cobra.Command {
	e := engine.New()
	e.Register(tmux.New())
	e.Register(kitty.New())

	cmd := &cobra.Command{
		Use:          "sesh",
		Short:        "Project-aware workspace orchestrator (tmux v0.1)",
		Version:      Version,
		SilenceUsage: true,
	}
	cmd.AddCommand(
		newUpCmd(e),
		newDownCmd(e),
		newLsCmd(),
		newEditCmd(),
		newNewCmd(),
		newDeleteCmd(),
		newDebugCmd(e),
		newCaptureCmd(e),
		newLocalCmd(e),
		newValidateCmd(e),
		newInitCmd(),
	)
	return cmd
}
