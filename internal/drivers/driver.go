// Package drivers defines the Driver interface used by the engine.
// Concrete implementations live in subpackages (tmux, kitty, ...).
package drivers

import (
	"context"

	"github.com/ijcd/sesh/internal/spec"
)

type ctxKey int

const socketHintKey ctxKey = iota

// WithSocketHint attaches a kitty socket path to ctx so that the kitty driver
// can read it without mutating os.Setenv. Ignored by non-kitty drivers.
func WithSocketHint(ctx context.Context, socket string) context.Context {
	return context.WithValue(ctx, socketHintKey, socket)
}

// SocketHintFromContext returns the socket hint set by WithSocketHint, if any.
func SocketHintFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(socketHintKey).(string)
	return v, ok && v != ""
}

// Status reports whether a driver currently has a session/workspace
// for the given project name.
type Status string

const (
	StatusUnknown   Status = "unknown"
	StatusNotExists Status = "not_exists"
	StatusExists    Status = "exists"
)

// Driver is what every backend implements (tmux v0.1; kitty/wezterm later).
type Driver interface {
	Name() string
	Up(ctx context.Context, p *spec.Project) error
	Down(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (Status, error)

	// Capture snapshots the live state into a draft *spec.Project. May return
	// (nil, nil) for drivers that don't support capture.
	Capture(ctx context.Context) (*spec.Project, error)

	// DryRun returns the exact shell commands Up would execute, as strings.
	// Used by `sesh debug`.
	DryRun(p *spec.Project) ([]string, error)

	// AttachCommand returns the shell command a parent driver should run to
	// attach to (host) this driver's container. Returns error if the driver
	// has no externally-attachable concept (e.g., kitty has no detached
	// sessions). Used by engine for cross-driver tab dispatch.
	AttachCommand(p *spec.Project) (string, error)

	// Validate runs driver-specific structural checks (layouts, etc.) and
	// returns any violations. Engine-level validation (containment, schema)
	// is separate; this is for driver internals.
	Validate(p *spec.Project) []error
}
