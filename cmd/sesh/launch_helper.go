package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ijcd/sesh/internal/drivers/kitty/launch"
	"github.com/ijcd/sesh/internal/spec"
	"github.com/ijcd/sesh/internal/state"
)

// needsLaunch reports whether the project requires sesh to spawn a fresh
// kitty before proceeding. True only when: driver is kitty, KITTY_LISTEN_ON
// is unset, AND the user passed --launch.
func needsLaunch(p *spec.Project, launchFlag bool) bool {
	if p.Driver != "kitty" {
		return false
	}
	if os.Getenv("KITTY_LISTEN_ON") != "" {
		return false
	}
	return launchFlag
}

// requiresKitty returns an error if the project needs kitty access but
// none is available.
func requiresKitty(p *spec.Project, launchFlag bool) error {
	if p.Driver != "kitty" {
		return nil
	}
	if os.Getenv("KITTY_LISTEN_ON") != "" {
		return nil
	}
	if launchFlag {
		return nil
	}
	return fmt.Errorf("kitty driver requires running inside kitty (KITTY_LISTEN_ON unset). Run again with --launch to spawn a new kitty for this project")
}

// performLaunch spawns kitty for the project, waits for the socket,
// sets KITTY_LISTEN_ON, and persists the launch state. Returns nil on
// success.
func performLaunch(ctx context.Context, p *spec.Project) error {
	stateDir, err := stateBaseDir()
	if err != nil {
		return err
	}
	sockPath, err := launch.SocketPathFor(p.Name, stateDir)
	if err != nil {
		return err
	}
	pid, err := launch.SpawnKitty(sockPath)
	if err != nil {
		return err
	}
	if err := launch.WaitForSocket(ctx, sockPath, 5*time.Second); err != nil {
		return fmt.Errorf("kitty did not become ready: %w", err)
	}
	os.Setenv("KITTY_LISTEN_ON", "unix:"+sockPath) //nolint:errcheck

	statePath, err := state.DefaultPath()
	if err != nil {
		return err
	}
	s, err := state.Load(statePath)
	if err != nil {
		return err
	}
	s.Set(p.Name, state.LaunchEntry{
		Socket: sockPath, Pid: pid, LaunchedAt: time.Now(),
	})
	return s.Save(statePath)
}

// stateBaseDir returns the base directory for sesh state files:
// $XDG_STATE_HOME/sesh, falling back to ~/.local/state/sesh.
func stateBaseDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return xdg + "/sesh", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + "/.local/state/sesh", nil
}
