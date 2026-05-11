package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveInclude_SingleInclude_HooksMerge verifies that a child's hooks.pre
// is appended after the template's hooks.pre (template body first, child body last)
// and that the Include field is cleared after resolution.
func TestResolveInclude_SingleInclude_HooksMerge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// write the template
	tmplDir := filepath.Join(dir, "sesh", "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "base.yml"), []byte("hooks:\n  pre: [\"template-hook\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// write the child project that includes the template
	projDir := filepath.Join(dir, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(projDir, "myproject.yml")
	if err := os.WriteFile(childPath, []byte("include: base\nhooks:\n  pre: [\"child-hook\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	child, err := LoadFile(childPath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResolveInclude(child, childPath)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"template-hook", "child-hook"}
	if len(out.Hooks.Pre) != len(want) {
		t.Fatalf("Hooks.Pre = %v, want %v", out.Hooks.Pre, want)
	}
	for i, v := range want {
		if out.Hooks.Pre[i] != v {
			t.Errorf("Hooks.Pre[%d] = %q, want %q", i, out.Hooks.Pre[i], v)
		}
	}
	if len(out.Include) != 0 {
		t.Errorf("Include not cleared after resolve: %v", out.Include)
	}
}

// TestResolveInclude_TwoIncludes_OrderPreserved verifies that two includes are
// merged left-to-right: left-template hooks, then right-template hooks, then
// child body last.
func TestResolveInclude_TwoIncludes_OrderPreserved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	tmplDir := filepath.Join(dir, "sesh", "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "left.yml"), []byte("hooks:\n  pre: [\"left-hook\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "right.yml"), []byte("hooks:\n  pre: [\"right-hook\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projDir := filepath.Join(dir, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(projDir, "combo.yml")
	if err := os.WriteFile(childPath, []byte("include: [left, right]\nhooks:\n  pre: [\"child-hook\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	child, err := LoadFile(childPath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResolveInclude(child, childPath)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"left-hook", "right-hook", "child-hook"}
	if len(out.Hooks.Pre) != len(want) {
		t.Fatalf("Hooks.Pre = %v, want %v", out.Hooks.Pre, want)
	}
	for i, v := range want {
		if out.Hooks.Pre[i] != v {
			t.Errorf("Hooks.Pre[%d] = %q, want %q", i, out.Hooks.Pre[i], v)
		}
	}
	if len(out.Include) != 0 {
		t.Errorf("Include not cleared after resolve: %v", out.Include)
	}
}
