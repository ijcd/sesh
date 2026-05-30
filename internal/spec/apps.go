package spec

import (
	"strconv"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/ijcd/sesh/internal/plugins"
)

// App is one entry in a project's `apps:` list — the envelope core reads,
// plus the opaque per-plugin config block routed to Plugin.New.
type App struct {
	Plugin   string            // required; names the registered plugin
	ID       string            // optional in YAML; auto-indexed at parse
	Optional bool              // soft-fail if true; default false (hard-fail)
	Drop     bool              // merge sentinel: child entry removes inherited match by Key
	Raw      plugins.RawConfig // opaque remainder for the plugin
}

// NewApp constructs an App with a non-nil Raw (a no-op RawConfig). Use for
// programmatic construction in tests, defaults, or migrations — the YAML
// path already populates Raw via UnmarshalYAML.
func NewApp(plugin, id string) App {
	return App{
		Plugin: plugin,
		ID:     id,
		Raw:    plugins.NewRawConfig(nil),
	}
}

// Key returns a stable identity for merge-by-key. First sighting of a plugin
// with no explicit id returns just the plugin name; subsequent same-plugin
// entries without an id get auto-indexed IDs ("2", "3", ...) at parse time.
//
//	emacs (1st, no id)     → ID="",     Key="emacs"
//	emacs (2nd, no id)     → ID="2",    Key="emacs#2"
//	emacs (id: main)       → ID="main", Key="emacs#main"
func (a App) Key() string {
	if a.ID == "" {
		return a.Plugin
	}
	return a.Plugin + "#" + a.ID
}

// UnmarshalYAML pulls envelope fields out of the entry node and stashes
// the full node as Raw. The plugin's struct ignores envelope fields it
// doesn't declare (goccy default behavior), so a stripped-node trick is
// unnecessary for v0.4.
func (a *App) UnmarshalYAML(node ast.Node) error {
	var env struct {
		Plugin   string `yaml:"plugin"`
		ID       string `yaml:"id,omitempty"`
		Optional bool   `yaml:"optional,omitempty"`
		Drop     bool   `yaml:"drop,omitempty"`
	}
	if err := yaml.NodeToValue(node, &env); err != nil {
		return err
	}
	a.Plugin = env.Plugin
	a.ID = env.ID
	a.Optional = env.Optional
	a.Drop = env.Drop
	a.Raw = plugins.NewRawConfig(node)
	return nil
}

// normalizeApps assigns auto-indexed IDs to apps[] entries that omit `id:`.
// Iteration is left-to-right. The first absent-id entry per plugin keeps
// ID="" (Key = plugin alone); subsequent absent-id entries for the same
// plugin get ID="2", "3", … Entries with explicit IDs are not touched and
// do not bump the absent-id counter.
func (p *Project) normalizeApps() {
	counts := make(map[string]int, len(p.Apps))
	for i := range p.Apps {
		a := &p.Apps[i]
		if a.ID != "" {
			continue
		}
		counts[a.Plugin]++
		if counts[a.Plugin] > 1 {
			a.ID = strconv.Itoa(counts[a.Plugin])
		}
	}
}
