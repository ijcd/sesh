package plugins

import "fmt"

// Registry holds Plugins in registration order. Order matters: dispatch
// and error messages are deterministic in the order plugins were added.
type Registry struct {
	plugins []Plugin
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register appends p. It returns an error naming the conflict if a plugin
// with the same Name is already registered.
func (r *Registry) Register(p Plugin) error {
	name := p.Name()
	for _, existing := range r.plugins {
		if existing.Name() == name {
			return fmt.Errorf("plugins: duplicate registration for %q", name)
		}
	}
	r.plugins = append(r.plugins, p)
	return nil
}

// Get returns the plugin registered under name, or (nil, false) if absent.
func (r *Registry) Get(name string) (Plugin, bool) {
	for _, p := range r.plugins {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

// Names returns plugin names in registration order. Never sorted.
func (r *Registry) Names() []string {
	out := make([]string, len(r.plugins))
	for i, p := range r.plugins {
		out[i] = p.Name()
	}
	return out
}
