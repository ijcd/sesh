// Package recipe embeds the default sesh.el policy file shipped alongside
// the emacs Lua plugin. The recipe is detachable: sesh's runtime never
// executes elisp, it only dispatches user-defined hooks. This package
// exists so `sesh init emacs` can stream the file to stdout for users to
// source.
package recipe

import _ "embed"

//go:embed sesh.el
var seshEL string

// Source returns the default sesh.el recipe contents. Callers print the
// returned string verbatim; sesh never executes elisp directly, it only
// dispatches the configured hooks via emacsclient.
func Source() string { return seshEL }
