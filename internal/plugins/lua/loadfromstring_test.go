package lua

import "fmt"

// loadFromString is a test helper: parses a Lua source string and
// registers each declared plugin via register(). Each registered plugin
// gets its own fresh LState (matching LoadAll's isolation guarantee).
func loadFromString(src string, register Register) error {
	names, err := stubDiscoverNames([]byte(src))
	if err != nil {
		return fmt.Errorf("lua: discover source: %w", err)
	}
	// Detect duplicate names within a single source (duplicate
	// registration is an error). The real sesh.register also
	// rejects them, but we'd otherwise lose that signal by only building one
	// plugin per unique name.
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			return fmt.Errorf("lua: load source: sesh.register: duplicate name %q", n)
		}
		seen[n] = true
	}
	for _, name := range names {
		p, err := buildPlugin(name, []byte(src))
		if err != nil {
			return fmt.Errorf("lua: load source: %w", err)
		}
		if err := register(p); err != nil {
			return err
		}
	}
	return nil
}
