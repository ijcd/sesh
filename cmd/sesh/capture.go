package main

import (
	"context"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/engine"
)

func newCaptureCmd(e *engine.Engine) *cobra.Command {
	return &cobra.Command{
		Use:   "capture <name>",
		Short: "Snapshot the current tmux session into a draft YAML on stdout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, drv := range e.Drivers() {
				if drv != "tmux" {
					continue
				}
				d := e.Driver(drv)
				if d == nil {
					continue
				}
				projects, err := d.Capture(context.Background())
				if err != nil {
					return err
				}
				if len(projects) == 0 {
					return fmt.Errorf("no current tmux session to capture")
				}
				p := projects[0]
				p.Name = args[0]
				out, err := yaml.Marshal(p)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(out)
				return err
			}
			return fmt.Errorf("no driver supports capture")
		},
	}
}
