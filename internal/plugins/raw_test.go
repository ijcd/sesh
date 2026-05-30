package plugins

import (
	"testing"

	"github.com/goccy/go-yaml/parser"
)

func TestRawConfig_DecodesPopulatedBlockIntoStruct(t *testing.T) {
	src := "hook: my-hook\nfiles:\n  - a\n  - b\n"
	f, err := parser.ParseBytes([]byte(src), 0)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if len(f.Docs) == 0 {
		t.Fatal("ParseBytes: no documents")
	}
	r := NewRawConfig(f.Docs[0].Body)

	var got struct {
		Hook  string   `yaml:"hook"`
		Files []string `yaml:"files"`
	}
	if err := r.Decode(&got); err != nil {
		t.Fatalf("Decode: unexpected error: %v", err)
	}
	if got.Hook != "my-hook" {
		t.Errorf("Hook: got %q, want %q", got.Hook, "my-hook")
	}
	if want := []string{"a", "b"}; len(got.Files) != 2 || got.Files[0] != want[0] || got.Files[1] != want[1] {
		t.Errorf("Files: got %v, want %v", got.Files, want)
	}
}

func TestRawConfig_AbsentBlockDecodesToZeroValue(t *testing.T) {
	r := NewRawConfig(nil)

	got := struct {
		Hook  string   `yaml:"hook"`
		Files []string `yaml:"files"`
	}{Hook: "untouched", Files: []string{"x"}}

	if err := r.Decode(&got); err != nil {
		t.Fatalf("Decode(nil node): unexpected error: %v", err)
	}
	// Per spec: nil node is a no-op — target is left untouched. The plugin
	// supplies its own zero value (or default) before calling Decode.
	if got.Hook != "untouched" {
		t.Errorf("Hook: got %q, want %q (Decode must not touch target on nil node)", got.Hook, "untouched")
	}
	if len(got.Files) != 1 || got.Files[0] != "x" {
		t.Errorf("Files: got %v, want [x] (Decode must not touch target on nil node)", got.Files)
	}
}

func TestRawConfig_TypeMismatchSurfacesError(t *testing.T) {
	src := "hook: my-hook\n"
	f, err := parser.ParseBytes([]byte(src), 0)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	r := NewRawConfig(f.Docs[0].Body)

	var got struct {
		Hook int `yaml:"hook"`
	}
	if err := r.Decode(&got); err == nil {
		t.Fatal("Decode: nil error, want type-mismatch error decoding string into int")
	}
}
