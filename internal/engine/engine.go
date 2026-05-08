// Package engine orchestrates project Up/Down across drivers.
package engine

import (
	"fmt"

	"github.com/ijcd/sesh/internal/drivers"
)

type Engine struct {
	drivers map[string]drivers.Driver
}

func New() *Engine { return &Engine{drivers: map[string]drivers.Driver{}} }

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
