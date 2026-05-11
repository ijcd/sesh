package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/config"
)

const newProjectTemplate = `# {{name}} — sesh project
driver: tmux
cwd: ~/work/{{name}}
tabs:
  - title: shell
  - title: dev
    cmd: echo 'replace me'
`

func newNewCmd() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new project file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dest, err := config.ResolveProjectPath(name)
			if err != nil {
				return err
			}
			if _, err := os.Stat(dest); err == nil {
				return fmt.Errorf("project %q already exists at %s", name, dest)
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			var body []byte
			if from != "" {
				src, err := config.ResolveProjectPath(from)
				if err != nil {
					return err
				}
				body, err = os.ReadFile(src)
				if err != nil {
					return fmt.Errorf("read --from %s: %w", from, err)
				}
			} else {
				body = []byte(strings.ReplaceAll(newProjectTemplate, "{{name}}", name))
			}
			if err := os.WriteFile(dest, body, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", dest)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Copy starting body from another project")
	return cmd
}
