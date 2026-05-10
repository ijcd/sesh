package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
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

			// If this project was launched via --launch, point KITTY_LISTEN_ON
			// at its tracked socket so Down can talk to that kitty.
			statePath, err := state.DefaultPath()
			if err == nil {
				if s, err2 := state.Load(statePath); err2 == nil {
					if entry, ok := s.Get(p.Name); ok {
						os.Setenv("KITTY_LISTEN_ON", "unix:"+entry.Socket)
						defer cleanupLaunch(s, p.Name, statePath, entry.Socket)
					}
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
