package kitty

import "fmt"

// DefaultLayout is what kitty driver uses when layout is unspecified
// and a tab has multiple panes.
const DefaultLayout = "splits"

var validLayouts = map[string]bool{
	"splits":     true,
	"tall":       true,
	"fat":        true,
	"grid":       true,
	"horizontal": true,
	"vertical":   true,
	"stack":      true,
}

// IsValidLayout reports whether name is a kitty layout (or empty for default).
func IsValidLayout(name string) bool {
	if name == "" {
		return true
	}
	return validLayouts[name]
}

// ValidateLayout returns nil for valid layouts, an error otherwise.
func ValidateLayout(name string) error {
	if IsValidLayout(name) {
		return nil
	}
	return fmt.Errorf("kitty: %q is not a valid layout (valid: splits, tall, fat, grid, horizontal, vertical, stack)", name)
}
