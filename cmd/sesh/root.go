package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/drivers/kitty"
	"github.com/ijcd/sesh/internal/drivers/tmux"
	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/plugins/emacs"
	luaplug "github.com/ijcd/sesh/internal/plugins/lua"
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
	// v0.5 spike: opt-in Lua plugin bridge. Registers embedded "emacs-lua"
	// (and any ~/.config/sesh/plugins/*.lua) alongside the Go "emacs"
	// plugin so both are available for A/B comparison. The Lua name is
	// distinct ("emacs-lua") so it does not collide; apps[] must opt in
	// by referencing it explicitly.
	if os.Getenv("SESH_USE_LUA_PLUGINS") != "" {
		if _, err := luaplug.LoadAll(e.RegisterPlugin); err != nil {
			panic(fmt.Errorf("sesh: load lua plugins: %w", err))
		}
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
