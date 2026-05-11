package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List configured projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dirPath, err := config.ResolveProjectPath("")
			if err != nil {
				return err
			}
			dir := filepath.Dir(dirPath)
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(cmd.OutOrStdout(), "(no projects yet — try `sesh new <name>`)")
					return nil
				}
				return err
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
					continue
				}
				fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSuffix(e.Name(), ".yml"))
			}
			return nil
		},
	}
}
