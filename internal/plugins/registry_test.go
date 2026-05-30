package plugins

import (
	"strings"
	"testing"
)

// fakePlugin is a minimal in-test Plugin implementation.
type fakePlugin struct{ name string }

func (f fakePlugin) Name() string                                  { return f.name }
func (f fakePlugin) New(env ProjectEnv, cfg RawConfig) (Instance, error) { return nil, nil }

func TestRegistry_GetReturnsRegisteredPlugin(t *testing.T) {
	r := NewRegistry()
	p := fakePlugin{name: "emacs"}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}

	got, ok := r.Get("emacs")
	if !ok {
		t.Fatal("Get(\"emacs\"): ok = false, want true")
	}
	if got.Name() != "emacs" {
		t.Errorf("Get: got Name() = %q, want %q", got.Name(), "emacs")
	}

	if _, ok := r.Get("missing"); ok {
		t.Error("Get(\"missing\"): ok = true, want false")
	}
}

func TestRegistry_RegisterRejectsDuplicateName(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakePlugin{name: "emacs"}); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	err := r.Register(fakePlugin{name: "emacs"})
	if err == nil {
		t.Fatal("second Register: nil error, want duplicate-name error")
	}
	if !strings.Contains(err.Error(), "emacs") {
		t.Errorf("error %q does not name the conflicting plugin %q", err.Error(), "emacs")
	}
}

func TestRegistry_NamesPreserveRegistrationOrder(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"a", "b", "c"} {
		if err := r.Register(fakePlugin{name: n}); err != nil {
			t.Fatalf("Register(%q): %v", n, err)
		}
	}

	got := r.Names()
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("Names(): got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Names()[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
