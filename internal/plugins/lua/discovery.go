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
	lua "github.com/yuin/gopher-lua"
)

//go:embed embed/*.lua
var embeddedFS embed.FS

// Register is the callback shape LoadAll uses to push each constructed
// LuaPlugin into the engine's registry. Matches engine.RegisterPlugin.
type Register func(plugins.Plugin) error

// LoadAll loads embedded Lua plugins, then user-dir plugins (from
// ~/.config/sesh/plugins/*.lua if it exists), and registers each via
// register(). User-dir plugins shadow embedded ones with the same
// registered name (last-write-wins on the (name → source-bytes) pair).
// Order is deterministic (embedded by sorted filename, then user-dir by
// sorted filename).
//
// Each constructed LuaPlugin owns its own *lua.LState; no globals are
// shared across plugins. Implementation is two-pass: discovery pass
// records (name → source-bytes); construction pass evaluates each
// source in its own fresh LState.
//
// Returns the names that were registered, in dispatch order
// (first-discovered ordering, user-dir shadowing preserves the
// original index).
func LoadAll(register Register) ([]string, error) {
	sources, order, err := discoverSources()
	if err != nil {
		return nil, err
	}
	return constructAndRegister(sources, order, register)
}

// discoverSources runs the discovery pass: load each .lua source into a
// throwaway LState equipped with a stub sesh.register that records the
// (name → source-bytes) pair into the returned map. Returns the map and
// the order in which names were first encountered (used as dispatch
// order; later shadows preserve the original index).
func discoverSources() (map[string][]byte, []string, error) {
	// Collect all sources in priority order: embedded first, then user-dir.
	type sourceFile struct {
		label string // for error messages
		src   []byte
	}
	var files []sourceFile

	embedFiles, err := listEmbedded()
	if err != nil {
		return nil, nil, fmt.Errorf("lua: list embedded plugins: %w", err)
	}
	for _, name := range embedFiles {
		src, err := embeddedFS.ReadFile(name)
		if err != nil {
			return nil, nil, fmt.Errorf("lua: read embedded %s: %w", name, err)
		}
		files = append(files, sourceFile{label: name, src: src})
	}

	userDir, err := userPluginDir()
	if err == nil {
		userFiles, err := listUserDir(userDir)
		if err != nil {
			return nil, nil, fmt.Errorf("lua: list user plugins: %w", err)
		}
		for _, path := range userFiles {
			src, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, fmt.Errorf("lua: read %s: %w", path, err)
			}
			files = append(files, sourceFile{label: path, src: src})
		}
	}

	sources := map[string][]byte{}
	var order []string
	for _, f := range files {
		names, err := stubDiscoverNames(f.src)
		if err != nil {
			return nil, nil, fmt.Errorf("lua: discover %s: %w", f.label, err)
		}
		for _, name := range names {
			if _, exists := sources[name]; !exists {
				order = append(order, name)
			}
			sources[name] = f.src
		}
	}
	return sources, order, nil
}

// stubDiscoverNames loads src in a throwaway LState whose sesh.register
// only records the name. Returns the names that the source registered.
// Other sesh.* APIs are stubbed as no-ops so top-level code doesn't
// crash (typical plugin top-level only calls sesh.register, but be
// defensive).
func stubDiscoverNames(src []byte) ([]string, error) {
	L := lua.NewState()
	defer L.Close()

	var names []string
	sesh := L.NewTable()
	L.SetField(sesh, "register", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		_ = L.CheckTable(2) // ensure shape, ignore body
		names = append(names, name)
		return 0
	}))
	// Stub log so plugin top-level log calls don't crash discovery.
	noop := L.NewFunction(func(*lua.LState) int { return 0 })
	logTbl := L.NewTable()
	L.SetField(logTbl, "info", noop)
	L.SetField(logTbl, "warn", noop)
	L.SetField(logTbl, "error", noop)
	L.SetField(sesh, "log", logTbl)
	// Stub file_exists so plugins that call it at top level (e.g., to pick
	// a fallback path before sesh.register) don't crash discovery. Returns
	// false; the real API runs during construction.
	L.SetField(sesh, "file_exists", L.NewFunction(func(L *lua.LState) int {
		_ = L.CheckString(1)
		L.Push(lua.LFalse)
		return 1
	}))
	L.SetGlobal("sesh", sesh)

	if err := L.DoString(string(src)); err != nil {
		return nil, err
	}
	return names, nil
}

// constructAndRegister builds one LuaPlugin per name in order, each with
// its own LState. For each name we evaluate the source bytes against a
// fresh state whose sesh.register captures the table and binds it to
// the LuaPlugin.
func constructAndRegister(sources map[string][]byte, order []string, register Register) ([]string, error) {
	var registered []string
	for _, name := range order {
		src := sources[name]
		p, err := buildPlugin(name, src)
		if err != nil {
			return registered, fmt.Errorf("lua: build %q: %w", name, err)
		}
		if err := register(p); err != nil {
			return registered, fmt.Errorf("lua: register %q: %w", name, err)
		}
		registered = append(registered, name)
	}
	return registered, nil
}

// buildPlugin creates a fresh LState with the real sesh.* API, evaluates
// src, and returns the LuaPlugin for the named registration. If src
// registers more than just `name` (e.g., a multi-plugin file shared
// across registrations), only the entry matching `name` is captured;
// others are skipped silently — they'll be built in their own pass.
func buildPlugin(name string, src []byte) (*LuaPlugin, error) {
	L := newState()
	r := &registrations{}
	setRegistrations(L, r)
	if err := L.DoString(string(src)); err != nil {
		L.Close()
		return nil, err
	}
	for _, e := range r.entries {
		if e.name == name {
			return &LuaPlugin{name: e.name, L: L, tbl: e.tbl}, nil
		}
	}
	L.Close()
	return nil, fmt.Errorf("source did not register %q", name)
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
