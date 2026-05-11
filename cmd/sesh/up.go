package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/engine"
)

// resolveUpName picks the project name from the CLI args or $SESH_PROJECT.
// Explicit arg always wins; env var is the fallback; error when neither is set.
func resolveUpName(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	if name := os.Getenv("SESH_PROJECT"); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("sesh up requires a project name or $SESH_PROJECT env var (set by direnv or shell)")
}

func newUpCmd(e *engine.Engine) *cobra.Command {
	var force bool
	var launchFlag bool
	cmd := &cobra.Command{
		Use:   "up [name]",
		Short: "Launch a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := resolveUpName(args)
			if err != nil {
				return err
			}
			p, err := config.Load(name, e.Drivers(), nil)
			if err != nil {
				return err
			}
			ctx := context.Background()
			return runProject(ctx, e, p, force, launchFlag)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Down + Up if a session already exists")
	cmd.Flags().BoolVar(&launchFlag, "launch", false, "Spawn a new kitty if not already inside one (kitty driver only)")
	return cmd
}
