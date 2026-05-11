package config

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/ijcd/sesh/internal/spec"
)

// Property: ResolveInclude with no includes is identity.
func TestResolveInclude_NoIncludeIsIdentity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		driver := rapid.SampledFrom([]string{"tmux", "kitty"}).Draw(t, "driver")
		p := &spec.Project{
			Name:   "test",
			Driver: driver,
			Tabs:   []spec.Tab{{Title: "shell"}},
		}
		out, err := ResolveInclude(p, "")
		if err != nil {
			t.Fatal(err)
		}
		if out != p {
			t.Errorf("ResolveInclude with no include should return input ptr unchanged")
		}
	})
}

// Property: After ResolveInclude, the Include field is empty (absorbed).
func TestResolveInclude_ClearsInclude(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Synthesize a project that includes nothing (empty list)
		p := &spec.Project{
			Driver: "tmux",
			Tabs:   []spec.Tab{{Title: "shell"}},
		}
		out, err := ResolveInclude(p, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Include) != 0 {
			t.Errorf("Include should be empty post-resolve, got %v", out.Include)
		}
	})
}
