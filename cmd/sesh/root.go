package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/drivers/kitty"
	"github.com/ijcd/sesh/internal/drivers/tmux"
	"github.com/ijcd/sesh/internal/engine"
	luaplug "github.com/ijcd/sesh/internal/plugins/lua"
)

const Version = "0.1.0-dev"

func newRootCmd() *cobra.Command {
	e := engine.New()
	e.Register(tmux.New())
	e.Register(kitty.New())
	// Lua plugin bridge. Registers embedded plugins (emacs, firefox) plus
	// any ~/.config/sesh/plugins/*.lua. The Go emacs package was removed in
	// v0.5; the Lua "emacs" plugin is now the sole emacs implementation.
	// A broken user plugin must NOT kill tier-1 commands (`sesh ls`, `edit`,
	// `init`, etc.); warn-and-continue, matching engine.Up's pattern for
	// optional plugin failures (internal/engine/up.go).
	if _, err := luaplug.LoadAll(e.RegisterPlugin); err != nil {
		fmt.Fprintf(os.Stderr, "sesh: warning: lua plugin load failed: %v\n", err)
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
