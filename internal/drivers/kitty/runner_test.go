package kitty

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKittenPath_PrefersPATH(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "kitten")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	p, err := kittenPath()
	if err != nil {
		t.Fatal(err)
	}
	if p != fake {
		t.Errorf("got %q, want %q", p, fake)
	}
}

func TestKittenPath_ErrorWhenMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("KITTEN_NO_FALLBACK", "1") // Disable fallback paths for this test
	_, err := kittenPath()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "kitten not found") {
		t.Errorf("got %v", err)
	}
}

type fakeRunner struct {
	runs       []string
	runErr     error
	captureOut string
	captureErr error
}

func (f *fakeRunner) Run(ctx context.Context, args ...string) error {
	f.runs = append(f.runs, "kitten "+strings.Join(args, " "))
	return f.runErr
}
func (f *fakeRunner) RunCapture(ctx context.Context, args ...string) (string, error) {
	return f.captureOut, f.captureErr
}

func TestExecRunner_PrefixesSocketArgs(t *testing.T) {
	// Verify that NewExecRunner injects --to <socket> before user args.
	// We can't actually exec without kitten; just check the wrapper logic.
	r := &execRunner{kittenPath: "/bin/echo", socket: "unix:/tmp/sock"}
	args := r.fullArgs([]string{"launch", "--type=tab"})
	want := []string{"@", "--to", "unix:/tmp/sock", "launch", "--type=tab"}
	if len(args) != len(want) {
		t.Fatalf("len mismatch: got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestExecRunner_ErrorsWhenSocketEmpty(t *testing.T) {
	r := &execRunner{kittenPath: "/bin/echo", socket: ""}
	err := r.Run(context.Background(), "ls")
	if err == nil {
		t.Fatal("expected error for empty socket")
	}
	if !strings.Contains(err.Error(), "KITTY_LISTEN_ON") {
		t.Errorf("got %v", err)
	}
	// To silence unused variable warning if errors stays imported:
	_ = errors.New
}
