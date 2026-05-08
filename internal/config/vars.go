package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ijcd/sesh/internal/spec"
)

// $${NAME} = literal escape; ${NAME} = variable.
// Go's regexp lacks lookbehind, so we use a two-pass sentinel trick:
// rewrite $${NAME} to a sentinel, expand ${NAME}, then restore the sentinel.
var (
	reEscaped = regexp.MustCompile(`\$\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	reVar     = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

// ExpandVars replaces ${NAME} in every string-typed leaf of p.
// Lookup order: p.Vars → env → error. $${NAME} → literal ${NAME}.
// env may be nil; when nil, os.LookupEnv is consulted.
func ExpandVars(p *spec.Project, env map[string]string) error {
	lookup := func(name string) (string, bool) {
		if v, ok := p.Vars[name]; ok {
			return v, true
		}
		if env != nil {
			v, ok := env[name]
			return v, ok
		}
		return os.LookupEnv(name)
	}
	for _, lf := range collectLeaves(p) {
		out, err := expandOne(*lf.ref, lookup, lf.path)
		if err != nil {
			return err
		}
		*lf.ref = out
	}
	return nil
}

func expandOne(s string, lookup func(string) (string, bool), keypath string) (string, error) {
	const sOpen, sClose = "\x00ESC_OPEN\x00", "\x00ESC_CLOSE\x00"
	s = reEscaped.ReplaceAllStringFunc(s, func(m string) string {
		name := reEscaped.FindStringSubmatch(m)[1]
		return sOpen + name + sClose
	})

	var firstErr error
	s = reVar.ReplaceAllStringFunc(s, func(m string) string {
		name := reVar.FindStringSubmatch(m)[1]
		v, ok := lookup(name)
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("undefined variable ${%s} at %s", name, keypath)
			}
			return m
		}
		return v
	})
	if firstErr != nil {
		return "", firstErr
	}
	s = strings.ReplaceAll(s, sOpen, "${")
	s = strings.ReplaceAll(s, sClose, "}")
	return s, nil
}

type leaf struct {
	path string
	ref  *string
}

func collectLeaves(p *spec.Project) []leaf {
	out := []leaf{
		{"cwd", &p.Cwd},
		{"session", &p.Session},
		{"startup_window", &p.StartupWindow},
		{"startup_pane", &p.StartupPane},
	}
	addStrings := func(path string, ss []string) {
		for i := range ss {
			out = append(out, leaf{fmt.Sprintf("%s[%d]", path, i), &ss[i]})
		}
	}
	addStrings("hooks.pre", p.Hooks.Pre)
	addStrings("hooks.post", p.Hooks.Post)
	addStrings("hooks.on_project_start", p.Hooks.OnStart)
	addStrings("hooks.on_project_stop", p.Hooks.OnStop)
	addStrings("pre_window", p.PreWindow)

	for ti := range p.Tabs {
		t := &p.Tabs[ti]
		prefix := fmt.Sprintf("tabs[%d]", ti)
		out = append(out,
			leaf{prefix + ".cwd", &t.Cwd},
			leaf{prefix + ".cmd", &t.Cmd},
			leaf{prefix + ".layout", &t.Layout},
		)
		addStrings(prefix+".pre_window", t.PreWindow)
		for pi := range t.Panes {
			pn := &t.Panes[pi]
			ppref := fmt.Sprintf("%s.panes[%d]", prefix, pi)
			out = append(out,
				leaf{ppref + ".cwd", &pn.Cwd},
				leaf{ppref + ".cmd", &pn.Cmd},
			)
		}
	}
	return out
}
