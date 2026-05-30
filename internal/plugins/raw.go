package plugins

import (
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// nodeRaw is the goccy-backed implementation of RawConfig. It wraps a YAML
// AST node and decodes it into the plugin's target struct on demand.
type nodeRaw struct {
	node ast.Node // nil when the block is absent — Decode is then a no-op
}

// NewRawConfig returns a RawConfig that decodes from the given YAML node.
// A nil node is legal: Decode is a no-op so the plugin keeps whatever value
// it pre-loaded into target (typically its zero value or a default).
func NewRawConfig(node ast.Node) RawConfig {
	return &nodeRaw{node: node}
}

// Decode unmarshals the underlying YAML node into into. If the node is nil
// (the apps[] entry had no opaque block), Decode is a no-op and into is
// left untouched — letting the plugin apply convention.
func (r *nodeRaw) Decode(into any) error {
	if r.node == nil {
		return nil
	}
	return yaml.NodeToValue(r.node, into)
}
