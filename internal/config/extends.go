package config

import (
	"fmt"
	"path/filepath"

	"github.com/ijcd/sesh/internal/spec"
)

// ResolveExtends walks the parent chain depth-first and returns the merged result.
// childPath is the absolute path of `child` (used as base for relative `extends:`
// values). Pass "" if the child wasn't loaded from disk.
func ResolveExtends(child *spec.Project, childPath string) (*spec.Project, error) {
	return resolveWithVisited(child, childPath, map[string]bool{absOrEmpty(childPath): true})
}

func resolveWithVisited(child *spec.Project, childPath string, visited map[string]bool) (*spec.Project, error) {
	if child.Extends == "" {
		return child, nil
	}
	parentPath, err := ResolveTemplatePath(child.Extends, filepath.Dir(childPath))
	if err != nil {
		return nil, fmt.Errorf("resolve extends %q: %w", child.Extends, err)
	}
	parentAbs, err := filepath.Abs(parentPath)
	if err != nil {
		return nil, err
	}
	if visited[parentAbs] {
		return nil, fmt.Errorf("extends cycle detected at %s", parentAbs)
	}
	visited[parentAbs] = true

	parent, err := LoadFile(parentPath)
	if err != nil {
		return nil, fmt.Errorf("load extends parent %q: %w", child.Extends, err)
	}
	parentResolved, err := resolveWithVisited(parent, parentPath, visited)
	if err != nil {
		return nil, err
	}
	merged := Merge(parentResolved, child)
	merged.Extends = ""
	return merged, nil
}

func absOrEmpty(p string) string {
	if p == "" {
		return ""
	}
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}
