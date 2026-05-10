package kitty

import (
	"strings"
	"testing"
)

func TestValidLayouts(t *testing.T) {
	for _, name := range []string{"splits", "tall", "fat", "grid", "horizontal", "vertical", "stack"} {
		if !IsValidLayout(name) {
			t.Errorf("%q should be valid", name)
		}
	}
}

func TestInvalidLayouts(t *testing.T) {
	for _, name := range []string{"main-vertical", "tiled", "even-horizontal", "garbage", "splits "} {
		if IsValidLayout(name) {
			t.Errorf("%q should be invalid", name)
		}
	}
}

func TestEmptyLayoutIsValid(t *testing.T) {
	// Empty means "use default"; not an error.
	if !IsValidLayout("") {
		t.Error("empty layout should be valid (default)")
	}
}

func TestDefaultLayout(t *testing.T) {
	if DefaultLayout != "splits" {
		t.Errorf("DefaultLayout = %q, want splits", DefaultLayout)
	}
}

func TestValidateLayoutErrorMessage(t *testing.T) {
	err := ValidateLayout("main-vertical")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "main-vertical") {
		t.Errorf("error should name the bad layout: %v", err)
	}
	if !strings.Contains(err.Error(), "splits") {
		t.Errorf("error should suggest valid layouts: %v", err)
	}
}
