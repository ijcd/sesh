//go:build e2e_docs

package e2e

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/config"
)

// TestReadmeExamples_Validate extracts every YAML code block under the
// "## Examples" section of README.md and runs it through config.LoadFromPath.
// Catches doc rot — when an example references a deprecated key or a removed
// feature, this test fails.
func TestReadmeExamples_Validate(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	examples := extractYAMLExamples(t, f)
	if len(examples) < 5 {
		t.Fatalf("expected at least 5 README examples, got %d", len(examples))
	}

	for i, body := range examples {
		t.Run(filepath.Base(body.label), func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "example.yml")
			if err := os.WriteFile(path, []byte(body.content), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XDG_CONFIG_HOME", tmp)
			_, err := config.LoadFromPath(path, []string{"tmux", "kitty"}, nil)
			if err != nil {
				t.Errorf("example #%d (%s) failed validation: %v\n---\n%s\n---", i, body.label, err, body.content)
			}
		})
	}
}

type yamlExample struct {
	label   string
	content string
}

// extractYAMLExamples walks the markdown looking for ```yaml code fences
// inside the "## Examples" section. Returns the YAML body and the heading
// label of the example.
func extractYAMLExamples(t *testing.T, r *os.File) []yamlExample {
	t.Helper()
	var out []yamlExample
	scanner := bufio.NewScanner(r)
	inExamples := false
	inYAML := false
	var currentLabel string
	var buf strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "## "):
			inExamples = strings.HasPrefix(line, "## Examples")
		case inExamples && strings.HasPrefix(line, "### "):
			currentLabel = strings.TrimPrefix(line, "### ")
		case inExamples && strings.HasPrefix(line, "```yaml"):
			inYAML = true
			buf.Reset()
		case inExamples && inYAML && strings.HasPrefix(line, "```"):
			inYAML = false
			// Skip examples that don't represent full project files
			// (e.g., template fragments, partial snippets).
			content := buf.String()
			if !looksLikeProject(content) {
				continue
			}
			out = append(out, yamlExample{label: currentLabel, content: content})
		case inExamples && inYAML:
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	return out
}

// looksLikeProject filters out fragment / template snippets — only validate
// content that looks like a full project (has both driver:, tabs:, and cwd:).
// Excludes examples that use include: (template references) or lack cwd:
// (template definitions that are meant to be inherited, not run standalone).
func looksLikeProject(yaml string) bool {
	hasDriver := strings.Contains(yaml, "driver:")
	hasTabs := strings.Contains(yaml, "tabs:")
	hasCwd := strings.Contains(yaml, "cwd:")
	hasInclude := strings.Contains(yaml, "include:")
	// Only validate self-contained projects: must have driver, tabs, and cwd;
	// must not use include: (which requires external template files).
	return hasDriver && hasTabs && hasCwd && !hasInclude
}
