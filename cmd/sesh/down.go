package main

import (
	"context"
	"fmt"
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
			ctx := context.Background()

			// If this project was launched via --launch, thread its tracked socket
			// into ctx so the kitty driver can talk to that instance without
			// mutating os.Setenv.
			statePath, err := state.DefaultPath()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not resolve state path: %v; --launch cleanup skipped\n", err)
			} else {
				s, err := state.Load(statePath)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read state.json: %v; --launch cleanup skipped\n", err)
				} else if entry, ok := s.Get(p.Name); ok {
					ctx = drivers.WithSocketHint(ctx, "unix:"+entry.Socket)
					defer cleanupLaunch(s, p.Name, statePath, entry.Socket)
				}
			}

			return e.Down(ctx, p)
		},
	}
}

func cleanupLaunch(s *state.Store, name, statePath, socket string) {
	s.Delete(name)
	_ = s.Save(statePath)
	_ = os.Remove(socket)
}
