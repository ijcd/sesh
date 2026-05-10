package kitty

import (
	"regexp"
	"strings"
)

// ProjectTabTitle returns the canonical "<project>:<tab>" tab title.
func ProjectTabTitle(project, tab string) string {
	return project + ":" + tab
}

// escapeMatch escapes regex metacharacters and colons for kitty --match syntax.
func escapeMatch(s string) string {
	quoted := regexp.QuoteMeta(s)
	return strings.ReplaceAll(quoted, ":", "\\:")
}

// MatchTabTitle returns a kitten --match expression for an exact tab title.
func MatchTabTitle(s string) string {
	return "tab_title:^" + escapeMatch(s) + "$"
}

// MatchTabTitlePrefix returns a --match for any tab whose title is "<prefix>:*".
func MatchTabTitlePrefix(prefix string) string {
	return "tab_title:^" + escapeMatch(prefix+":") + ".*$"
}

// MatchWindowTitle returns a --match expression for a kitty window (pane) title.
func MatchWindowTitle(s string) string {
	return "title:^" + escapeMatch(s) + "$"
}
