package lua

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ijcd/sesh/internal/plugins"
)

//go:embed embed/*.lua
var embeddedFS embed.FS

// Register is the callback shape LoadAll uses to push each constructed
// LuaPlugin into the engine's registry. Matches engine.RegisterPlugin.
type Register func(plugins.Plugin) error

// LoadAll loads embedded Lua plugins, then user-dir plugins (from
// ~/.config/sesh/plugins/*.lua if it exists), and registers each via
// register(). User-dir plugins shadow embedded ones with the same
// registered name (last-write-wins on the (name → table) pair before
// any Register call). Order is deterministic (embedded by sorted
// filename, then user-dir by sorted filename).
//
// Returns the names that were registered, in dispatch order.
func LoadAll(register Register) ([]string, error) {
	L := newState()
	r := &registrations{}
	setRegistrations(L, r)

	// Embedded plugins.
	embedFiles, err := listEmbedded()
	if err != nil {
		return nil, fmt.Errorf("lua: list embedded plugins: %w", err)
	}
	for _, name := range embedFiles {
		src, err := embeddedFS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("lua: read embedded %s: %w", name, err)
		}
		if err := L.DoString(string(src)); err != nil {
			return nil, fmt.Errorf("lua: load embedded %s: %w", name, err)
		}
	}

	// User-dir plugins.
	userDir, err := userPluginDir()
	if err == nil {
		userFiles, err := listUserDir(userDir)
		if err != nil {
			return nil, fmt.Errorf("lua: list user plugins: %w", err)
		}
		for _, path := range userFiles {
			if err := L.DoFile(path); err != nil {
				return nil, fmt.Errorf("lua: load %s: %w", path, err)
			}
		}
	}

	// Build LuaPlugins and register. User-dir registrations come later;
	// they shadow embedded ones because we register in order and treat
	// later-name as authoritative by removing the prior one.
	//
	// Spike note: plugins.Registry doesn't expose a Remove() method;
	// rather than add one, we deduplicate at the registrations[] level
	// here. Last entry wins.
	dedup := map[string]int{}
	for i := range r.entries {
		dedup[r.entries[i].name] = i
	}
	finalIdx := make([]int, 0, len(dedup))
	for _, i := range dedup {
		finalIdx = append(finalIdx, i)
	}
	sort.Ints(finalIdx) // stable dispatch order = first-registration index

	var names []string
	for _, i := range finalIdx {
		e := r.entries[i]
		p := &LuaPlugin{name: e.name, L: L, tbl: e.tbl}
		if err := register(p); err != nil {
			return names, fmt.Errorf("lua: register %q: %w", e.name, err)
		}
		names = append(names, e.name)
	}
	return names, nil
}

func listEmbedded() ([]string, error) {
	var out []string
	err := fs.WalkDir(embeddedFS, "embed", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".lua") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func userPluginDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sesh", "plugins"), nil
}

func listUserDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".lua") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

// loadFromString is a test helper: parses a Lua source string and
// registers each declared plugin via register(). One fresh LState per
// call (state is owned by the constructed LuaPlugins).
func loadFromString(src string, register Register) error {
	L := newState()
	r := &registrations{}
	setRegistrations(L, r)
	if err := L.DoString(src); err != nil {
		return fmt.Errorf("lua: load source: %w", err)
	}
	for _, e := range r.entries {
		p := &LuaPlugin{name: e.name, L: L, tbl: e.tbl}
		if err := register(p); err != nil {
			return err
		}
	}
	return nil
}
