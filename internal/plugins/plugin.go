// Package plugins defines the v0.4 plugin SPI: mechanism-only types core
// uses to register, configure, and drive non-terminal sidecars (editor,
// browser, comms). Each plugin owns its own config schema — including what
// it treats as config vs convention. Core ships the type spine and an
// ordered registry; concrete plugins live in their own packages.
package plugins

import "context"

// Plugin is the factory registered at startup. It owns its config schema
// entirely — including what it treats as config vs convention/default.
type Plugin interface {
	// Name is the unique registry key (e.g. "emacs").
	Name() string
	// New decodes cfg into the plugin's internal struct and binds it to env,
	// returning one Instance per apps[] entry per `sesh up`.
	New(env ProjectEnv, cfg RawConfig) (Instance, error)
}

// Instance is one configured unit per apps[] entry per `sesh up`.
type Instance interface {
	Up(ctx context.Context) error
	Down(ctx context.Context) error
	Validate() []error
	DryRun() ([]string, error)
}

// ProjectEnv is the irreducible identity core promises every plugin.
// Grows only on demonstrated need.
type ProjectEnv struct {
	Name string
	Cwd  string
}

// RawConfig is the opaque per-plugin config block. The plugin unmarshals
// into its own struct via Decode, never touching the YAML library directly.
// The concrete implementation lands in T2.
type RawConfig interface {
	Decode(into any) error
}
