package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/engine"
	"github.com/ijcd/sesh/internal/spec"
)

type captureResult struct {
	driver   string
	projects []*spec.Project
}

func newCaptureCmd(e *engine.Engine) *cobra.Command {
	var flagList bool
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "capture [name]",
		Short: "Snapshot running sessions into draft YAML on stdout",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagAll && len(args) > 0 {
				return fmt.Errorf("--all and positional name are mutually exclusive")
			}

			// no args, no flags → default to --list
			if !flagAll && len(args) == 0 {
				flagList = true
			}

			// collect from all drivers (sorted for determinism)
			driverNames := e.Drivers()
			sort.Strings(driverNames)

			var results []captureResult
			for _, drv := range driverNames {
				d := e.Driver(drv)
				if d == nil {
					continue
				}
				projects, err := d.Capture(context.Background())
				if err != nil {
					return fmt.Errorf("driver %s: %w", drv, err)
				}
				results = append(results, captureResult{driver: drv, projects: projects})
			}

			switch {
			case flagList:
				return runCaptureList(cmd, results)
			case flagAll:
				return runCaptureAll(cmd, results)
			default:
				return runCaptureLegacy(cmd, args[0], results)
			}
		},
	}

	cmd.Flags().BoolVar(&flagList, "list", false, "Print a summary of available captures")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Emit all captures as multi-doc YAML")
	return cmd
}

func runCaptureList(cmd *cobra.Command, results []captureResult) error {
	total := 0
	for _, r := range results {
		total += len(r.projects)
	}
	if total == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "no captures available")
		return err
	}
	var sb strings.Builder
	for _, r := range results {
		if len(r.projects) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "%s:\n", r.driver)
		for _, p := range r.projects {
			tabWord := "tab"
			if len(p.Tabs) != 1 {
				tabWord = "tabs"
			}
			name := p.Name
			if name == "" {
				name = "(unnamed)"
			}
			fmt.Fprintf(&sb, "  - %s (%d %s)\n", name, len(p.Tabs), tabWord)
		}
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), sb.String())
	return err
}

func runCaptureAll(cmd *cobra.Command, results []captureResult) error {
	w := cmd.OutOrStdout()
	for _, r := range results {
		for _, p := range r.projects {
			label := fmt.Sprintf("captured from %s session: %s", r.driver, p.Name)
			out, err := yaml.Marshal(p)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "---\n# %s\n%s", label, out); err != nil {
				return err
			}
		}
	}
	return nil
}

func runCaptureLegacy(cmd *cobra.Command, name string, results []captureResult) error {
	// First item across all drivers (drivers already sorted).
	for _, r := range results {
		if len(r.projects) == 0 {
			continue
		}
		p := r.projects[0]
		p.Name = name
		out, err := yaml.Marshal(p)
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(out)
		return err
	}
	return fmt.Errorf("no captures available")
}
