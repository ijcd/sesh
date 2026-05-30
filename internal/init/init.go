// Package init renders init snippets for `sesh init <target>`. Targets are
// shells (zsh/bash/fish) that eval the discovery hook, plus `emacs` which
// emits the default policy recipe shipped with the emacs plugin.
package init

import (
	_ "embed"
	"fmt"

	"github.com/ijcd/sesh/internal/plugins/emacs/recipe"
)

//go:embed scripts/zsh.sh
var zshScript string

//go:embed scripts/bash.sh
var bashScript string

//go:embed scripts/fish.sh
var fishScript string

// Render returns the snippet for target. Shell targets (zsh/bash/fish) emit
// the discovery hook to be eval'd from rc; the emacs target emits the
// default sesh.el recipe to be (load-file ...)'d from init.el.
func Render(target string) (string, error) {
	switch target {
	case "zsh":
		return zshScript, nil
	case "bash":
		return bashScript, nil
	case "fish":
		return fishScript, nil
	case "emacs":
		return recipe.Source(), nil
	default:
		return "", fmt.Errorf("unknown init target %q (supported: zsh, bash, fish, emacs)", target)
	}
}
