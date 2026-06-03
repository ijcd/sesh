package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/drivers/kitty/launch"
	"github.com/ijcd/sesh/internal/spec"
	"github.com/ijcd/sesh/internal/state"
	"github.com/ijcd/sesh/internal/xdg"
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

// performLaunch spawns kitty for the project, waits for the socket, and
// persists the launch state. Returns the enriched ctx carrying the kitty
// socket via drivers.WithSocketHint so the kitty driver can talk to the
// freshly-spawned instance without mutating os.Setenv. Mirrors the ctx
// threading already used by `sesh down`.
func performLaunch(ctx context.Context, p *spec.Project) (context.Context, error) {
	stateDir, err := stateBaseDir()
	if err != nil {
		return ctx, err
	}
	sockPath, err := launch.SocketPathFor(p.Name, stateDir)
	if err != nil {
		return ctx, err
	}
	pid, err := launch.SpawnKitty(sockPath)
	if err != nil {
		return ctx, err
	}
	if err := launch.WaitForSocket(ctx, sockPath, 5*time.Second); err != nil {
		return ctx, fmt.Errorf("kitty did not become ready: %w", err)
	}
	ctx = drivers.WithSocketHint(ctx, "unix:"+sockPath)

	statePath, err := state.DefaultPath()
	if err != nil {
		return ctx, err
	}
	s, err := state.Load(statePath)
	if err != nil {
		return ctx, err
	}
	s.Set(p.Name, state.LaunchEntry{
		Socket: sockPath, Pid: pid, LaunchedAt: time.Now(),
	})
	if err := s.Save(statePath); err != nil {
		return ctx, err
	}
	return ctx, nil
}

// stateBaseDir returns the base directory for sesh state files:
// $XDG_STATE_HOME/sesh, falling back to ~/.local/state/sesh.
func stateBaseDir() (string, error) {
	sh, err := xdg.StateHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(sh, "sesh"), nil
}
