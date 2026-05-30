package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/drivers/kitty"
	"github.com/ijcd/sesh/internal/drivers/tmux"
	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/plugins/emacs"
)

const Version = "0.1.0-dev"

func newRootCmd() *cobra.Command {
	e := engine.New()
	e.Register(tmux.New())
	e.Register(kitty.New())
	if err := e.RegisterPlugin(emacs.New()); err != nil {
		// Duplicate registration of a builtin plugin is a programmer error
		// (root setup is the only call site). Panic mirrors how cobra
		// itself treats duplicate command registration.
		panic(fmt.Errorf("sesh: register emacs plugin: %w", err))
	}

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
