package config

import (
	"fmt"
	"path/filepath"

	"github.com/ijcd/sesh/internal/spec"
)

// ResolveInclude walks the include: list left-to-right (depth-first within each
// entry's own include chain) and returns the merged result.
// childPath is the absolute path of `child` (used as base for relative include
// values). Pass "" if the child wasn't loaded from disk.
func ResolveInclude(child *spec.Project, childPath string) (*spec.Project, error) {
	if child.Extends != "" {
		return nil, fmt.Errorf("extends: is no longer supported (sesh v0.3+); rename to include:")
	}
	if len(child.Include) == 0 {
		return child, nil
	}
	return resolveIncludeWithVisited(child, childPath, map[string]bool{absOrEmpty(childPath): true})
}

func resolveIncludeWithVisited(child *spec.Project, childPath string, visited map[string]bool) (*spec.Project, error) {
	if len(child.Include) == 0 {
		return child, nil
	}
	// Accumulator: starts as zero project; merge each include left-to-right; then merge child body last.
	acc := &spec.Project{}
	for _, inc := range child.Include {
		incPath, err := ResolveTemplatePath(inc, filepath.Dir(childPath))
		if err != nil {
			return nil, fmt.Errorf("resolve include %q: %w", inc, err)
		}
		incAbs, err := filepath.Abs(incPath)
		if err != nil {
			return nil, err
		}
		if visited[incAbs] {
			return nil, fmt.Errorf("include cycle detected at %s", incAbs)
		}
		// Branch the visited map per include so siblings don't poison each other.
		nextVisited := copyVisited(visited)
		nextVisited[incAbs] = true

		parent, err := LoadFile(incPath)
		if err != nil {
			return nil, fmt.Errorf("load include %q: %w", inc, err)
		}
		parentResolved, err := resolveIncludeWithVisited(parent, incPath, nextVisited)
		if err != nil {
			return nil, err
		}
		acc = Merge(acc, parentResolved)
	}
	merged := Merge(acc, child)
	merged.Include = nil
	return merged, nil
}

func copyVisited(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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
