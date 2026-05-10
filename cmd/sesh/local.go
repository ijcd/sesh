package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
	"github.com/ijcd/sesh/internal/engine"
)

func newLocalCmd(e *engine.Engine) *cobra.Command {
	var force bool
	var launchFlag bool
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Run ./.sesh.yml from the current directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			path := filepath.Join(cwd, ".sesh.yml")
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("no .sesh.yml in %s", cwd)
			}
			p, err := config.LoadFromPath(path, e.Drivers(), nil)
			if err != nil {
				return err
			}
			if err := requiresKitty(p, launchFlag); err != nil {
				return err
			}
			ctx := context.Background()
			if needsLaunch(p, launchFlag) {
				if err := performLaunch(ctx, p); err != nil {
					return err
				}
			}
			if err := e.Up(ctx, p, force); err != nil {
				return err
			}
			if p.Attach == nil || *p.Attach {
				return attachIfTmux(p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Down + Up if a session already exists")
	cmd.Flags().BoolVar(&launchFlag, "launch", false, "Spawn a new kitty if not already inside one (kitty driver only)")
	return cmd
}
