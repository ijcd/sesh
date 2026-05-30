package spec

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestApp_KeyOmitsSuffixWhenIdEmpty(t *testing.T) {
	a := App{Plugin: "emacs"}
	if got, want := a.Key(), "emacs"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}

func TestApp_KeyAppendsExplicitId(t *testing.T) {
	a := App{Plugin: "emacs", ID: "main"}
	if got, want := a.Key(), "emacs#main"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}

func TestApp_AutoIndexesSecondAbsentIdPerPlugin(t *testing.T) {
	in := []byte(`
apps:
  - plugin: emacs
    hook: a
  - plugin: emacs
    hook: b
`)
	var p Project
	if err := yaml.Unmarshal(in, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Apps) != 2 {
		t.Fatalf("len(apps) = %d, want 2", len(p.Apps))
	}
	if got, want := p.Apps[0].Key(), "emacs"; got != want {
		t.Errorf("Apps[0].Key() = %q, want %q", got, want)
	}
	if got, want := p.Apps[1].Key(), "emacs#2"; got != want {
		t.Errorf("Apps[1].Key() = %q, want %q", got, want)
	}
}

func TestApp_ExplicitIdNotAutoIndexed(t *testing.T) {
	in := []byte(`
apps:
  - plugin: emacs
  - plugin: emacs
    id: main
`)
	var p Project
	if err := yaml.Unmarshal(in, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Apps) != 2 {
		t.Fatalf("len(apps) = %d, want 2", len(p.Apps))
	}
	if got, want := p.Apps[0].Key(), "emacs"; got != want {
		t.Errorf("Apps[0].Key() = %q, want %q", got, want)
	}
	if got, want := p.Apps[1].Key(), "emacs#main"; got != want {
		t.Errorf("Apps[1].Key() = %q, want %q", got, want)
	}
}

func TestApp_RawCarriesOpaqueRemainder(t *testing.T) {
	in := []byte(`
apps:
  - plugin: emacs
    hook: foo
    files:
      - a
`)
	var p Project
	if err := yaml.Unmarshal(in, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Apps) != 1 {
		t.Fatalf("len(apps) = %d, want 1", len(p.Apps))
	}
	var got struct {
		Hook  string   `yaml:"hook"`
		Files []string `yaml:"files"`
	}
	if err := p.Apps[0].Raw.Decode(&got); err != nil {
		t.Fatalf("Raw.Decode: %v", err)
	}
	if got.Hook != "foo" {
		t.Errorf("Hook = %q, want %q", got.Hook, "foo")
	}
	if len(got.Files) != 1 || got.Files[0] != "a" {
		t.Errorf("Files = %v, want [a]", got.Files)
	}
}
