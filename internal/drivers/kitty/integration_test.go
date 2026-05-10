//go:build integration_kitty

package kitty

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func TestIntegration_KittenAvailable(t *testing.T) {
	if _, err := exec.LookPath("kitten"); err != nil {
		t.Skip("kitten not on PATH")
	}
}

func TestIntegration_LsFromRunningKitty(t *testing.T) {
	sock := os.Getenv("KITTY_LISTEN_ON_TEST")
	if sock == "" {
		t.Skip("KITTY_LISTEN_ON_TEST not set; skipping kitty integration")
	}
	t.Setenv("KITTY_LISTEN_ON", sock)
	d := New()
	p, err := d.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Skip("no OS windows in test kitty instance")
	}
	if p.Driver != "kitty" {
		t.Errorf("driver = %q, want kitty", p.Driver)
	}
}
