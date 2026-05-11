package config

import "github.com/ijcd/sesh/internal/spec"

// Merge applies the unified merge rules: scalars child-wins, hashes deep-merge,
// titled lists merge by title (drop:true removes), string lists append.
// Returns a fresh *spec.Project; inputs are not mutated.
func Merge(parent, child *spec.Project) *spec.Project {
	out := &spec.Project{
		Name:          coalesce(child.Name, parent.Name),
		Extends:       "", // cleared after merge — chain already resolved upstream
		Cwd:           coalesce(child.Cwd, parent.Cwd),
		Driver:        coalesce(child.Driver, parent.Driver),
		Session:       coalesce(child.Session, parent.Session),
		StartupWindow: coalesce(child.StartupWindow, parent.StartupWindow),
		StartupPane:   coalesce(child.StartupPane, parent.StartupPane),
		Attach:        coalesceBool(child.Attach, parent.Attach),
		Vars:          mergeMap(parent.Vars, child.Vars),
		Hooks:         mergeHooks(parent.Hooks, child.Hooks),
		PreWindow:     appendStrings(parent.PreWindow, child.PreWindow),
		Tabs:          mergeTabs(parent.Tabs, child.Tabs),
	}
	return out
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func coalesceBool(a, b *bool) *bool {
	if a != nil {
		return a
	}
	return b
}

func mergeMap(p, c map[string]string) map[string]string {
	if p == nil && c == nil {
		return nil
	}
	out := make(map[string]string, len(p)+len(c))
	for k, v := range p {
		out[k] = v
	}
	for k, v := range c {
		out[k] = v
	}
	return out
}

func mergeHooks(p, c spec.Hooks) spec.Hooks {
	return spec.Hooks{
		Pre:     appendStrings(p.Pre, c.Pre),
		Post:    appendStrings(p.Post, c.Post),
		OnStart: appendStrings(p.OnStart, c.OnStart),
		OnStop:  appendStrings(p.OnStop, c.OnStop),
	}
}

func appendStrings(p, c spec.StringList) spec.StringList {
	if len(p) == 0 && len(c) == 0 {
		return nil
	}
	out := make(spec.StringList, 0, len(p)+len(c))
	out = append(out, p...)
	out = append(out, c...)
	return out
}

// mergeByTitle merges two titled lists by title: child scalars win, Drop:true removes,
// child-only entries append. mergeOne is called for matched pairs.
func mergeByTitle[T any](parent, child []T, titleOf func(T) string, droppedOf func(T) bool, mergeOne func(T, T) T) []T {
	childByTitle := map[string]*T{}
	for i := range child {
		t := titleOf(child[i])
		childByTitle[t] = &child[i]
	}
	seen := map[string]bool{}
	out := make([]T, 0, len(parent)+len(child))

	for i := range parent {
		p := parent[i]
		seen[titleOf(p)] = true
		if c, ok := childByTitle[titleOf(p)]; ok {
			if droppedOf(*c) {
				continue
			}
			out = append(out, mergeOne(p, *c))
		} else {
			out = append(out, p)
		}
	}
	for i := range child {
		c := child[i]
		if seen[titleOf(c)] {
			continue
		}
		if droppedOf(c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func mergeTabs(parent, child []spec.Tab) []spec.Tab {
	return mergeByTitle(parent, child,
		func(t spec.Tab) string { return t.Title },
		func(t spec.Tab) bool { return t.Drop },
		mergeTab,
	)
}

func mergeTab(p, c spec.Tab) spec.Tab {
	return spec.Tab{
		Title:     p.Title,
		Cwd:       coalesce(c.Cwd, p.Cwd),
		Cmd:       coalesce(c.Cmd, p.Cmd),
		Driver:    coalesce(c.Driver, p.Driver),
		Layout:    coalesce(c.Layout, p.Layout),
		PreWindow: appendStrings(p.PreWindow, c.PreWindow),
		Panes:     mergePanes(p.Panes, c.Panes),
		Drop:      false,
	}
}

func mergePanes(parent, child []spec.Pane) []spec.Pane {
	return mergeByTitle(parent, child,
		func(p spec.Pane) string { return p.Title },
		func(p spec.Pane) bool { return p.Drop },
		mergePane,
	)
}

func mergePane(p, c spec.Pane) spec.Pane {
	return spec.Pane{
		Title: p.Title,
		Cwd:   coalesce(c.Cwd, p.Cwd),
		Cmd:   coalesce(c.Cmd, p.Cmd),
		Drop:  false,
	}
}
