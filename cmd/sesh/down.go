package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/state"
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

			// Classify cleanup state up front: did this project come up via
			// --launch (tracked in state.json)? If so, thread its socket into
			// ctx so the kitty driver talks to that instance without mutating
			// os.Setenv, and defer cleanup of the state entry + socket file.
			ctx, cleanup := resolveLaunchCleanup(cmd.ErrOrStderr(), p.Name)
			if cleanup != nil {
				defer cleanup()
			}

			return e.Down(ctx, p)
		},
	}
}

// resolveLaunchCleanup looks up the project in state.json and returns
// (ctx-with-socket-hint, deferred-cleanup) when a launched entry is found.
// Any state-layer error is logged once and the function returns
// (context.Background(), nil) — Down still runs, just without launch cleanup.
func resolveLaunchCleanup(errOut io.Writer, projectName string) (context.Context, func()) {
	ctx := context.Background()

	statePath, err := state.DefaultPath()
	if err != nil {
		fmt.Fprintf(errOut, "warning: could not resolve state path: %v; --launch cleanup skipped\n", err)
		return ctx, nil
	}
	s, err := state.Load(statePath)
	if err != nil {
		fmt.Fprintf(errOut, "warning: could not read state.json: %v; --launch cleanup skipped\n", err)
		return ctx, nil
	}
	entry, ok := s.Get(projectName)
	if !ok {
		return ctx, nil
	}
	ctx = drivers.WithSocketHint(ctx, "unix:"+entry.Socket)
	return ctx, func() { cleanupLaunch(s, projectName, statePath, entry.Socket) }
}

func cleanupLaunch(s *state.Store, name, statePath, socket string) {
	s.Delete(name)
	_ = s.Save(statePath)
	_ = os.Remove(socket)
}
