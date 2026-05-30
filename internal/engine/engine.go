// Package engine orchestrates project Up/Down across drivers.
package engine

import (
	"fmt"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/plugins"
)

type Engine struct {
	drivers  map[string]drivers.Driver
	registry *plugins.Registry
}

func New() *Engine {
	return &Engine{
		drivers:  map[string]drivers.Driver{},
		registry: plugins.NewRegistry(),
	}
}

func (e *Engine) Register(d drivers.Driver) { e.drivers[d.Name()] = d }

func (e *Engine) Drivers() []string {
	out := make([]string, 0, len(e.drivers))
	for n := range e.drivers {
		out = append(out, n)
	}
	return out
}

func (e *Engine) driverFor(name string) (drivers.Driver, error) {
	d, ok := e.drivers[name]
	if !ok {
		return nil, fmt.Errorf("driver %q not registered", name)
	}
	return d, nil
}

// Driver returns the registered driver by name, or nil.
func (e *Engine) Driver(name string) drivers.Driver { return e.drivers[name] }

// RegisterPlugin adds p to the engine's plugin registry. Plugin Up/Down
// dispatch follows project Apps list order; the registry's registration
// order is only authoritative for Plugins() and tie-breaking in error
// messages.
func (e *Engine) RegisterPlugin(p plugins.Plugin) error {
	return e.registry.Register(p)
}

// Plugins returns the names of registered plugins in registration order.
func (e *Engine) Plugins() []string { return e.registry.Names() }
