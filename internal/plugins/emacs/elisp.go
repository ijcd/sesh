// Package emacs holds pure builders that emit elisp source strings for the
// emacs plugin lifecycle. The builders never decide what "opening a project"
// means in emacs — they only emit a call to a user-defined hook with the
// project identity (and optionally the files to surface). The hook itself
// — `sesh-open-project`, `sesh-close-project`, or whatever name the user
// configured — lives in the user's emacs config and is where layout policy
// (desktop.el, project.el, tab-bar, perspective.el, …) belongs.
//
// Mechanism-not-policy: this file emits *calls*, never behavior.
package emacs

import (
	"path/filepath"
	"strings"
)

// BuildOpenForm returns an elisp form that invokes the user-defined open
// hook with the project name, cwd, and (optionally) a quoted list of
// absolute file paths.
//
// Files are absolutized against cwd: bare names like "README.md" become
// "<cwd>/README.md"; already-absolute paths are passed through unchanged.
// All strings are elisp-quoted so backslashes and double quotes survive.
//
// When files is empty/nil the form omits the third arg entirely (two-arg
// call) so the hook can distinguish "no files requested" from "empty list".
func BuildOpenForm(hook, name, cwd string, files []string) string {
	if len(files) == 0 {
		return "(" + hook + " " + quote(name) + " " + quote(cwd) + ")"
	}
	parts := make([]string, len(files))
	for i, f := range files {
		if !filepath.IsAbs(f) {
			f = filepath.Join(cwd, f)
		}
		parts[i] = quote(f)
	}
	return "(" + hook + " " + quote(name) + " " + quote(cwd) + " '(" + strings.Join(parts, " ") + "))"
}

// BuildCloseForm returns an elisp form that invokes the user-defined close
// hook with the project name. By convention the hook name mirrors the open
// hook (e.g., "sesh-close-project" pairs with "sesh-open-project").
func BuildCloseForm(hook, name string) string {
	return "(" + hook + " " + quote(name) + ")"
}

// quote wraps s in a double-quoted elisp string literal, escaping backslash
// and double-quote. Order matters: backslash must be escaped before quote
// so the inserted backslashes from the quote escape aren't double-escaped.
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
