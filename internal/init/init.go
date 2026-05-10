// Package init renders shell-specific discovery hook scripts.
package init

import (
	_ "embed"
	"fmt"
)

//go:embed scripts/zsh.sh
var zshScript string

//go:embed scripts/bash.sh
var bashScript string

//go:embed scripts/fish.sh
var fishScript string

// Render returns the shell-specific init snippet to be eval'd from the user's rc.
func Render(shell string) (string, error) {
	switch shell {
	case "zsh":
		return zshScript, nil
	case "bash":
		return bashScript, nil
	case "fish":
		return fishScript, nil
	default:
		return "", fmt.Errorf("unknown shell %q (supported: zsh, bash, fish)", shell)
	}
}
