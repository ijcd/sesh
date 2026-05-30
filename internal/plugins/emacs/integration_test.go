//go:build integration_emacs

// Real-emacs integration test for the emacs plugin. Gated `integration_emacs`
// so plain `go test ./...` keeps skipping it (no emacs dependency on CI).
//
// Coverage: pre-spawn a uniquely-named emacs daemon, install test hooks that
// record their argv into a daemon-side global, then run plugin Up/Down against
// the daemon via the real Plugin factory + RawConfig decode path. Asserts the
// daemon-side global was mutated to the expected (:open …) / (:close …) shape.
//
// The plugin's own daemon-spawn fallback (`emacs --daemon=<name>` when the
// probe fails) is intentionally *not* exercised here: `emacs --daemon=...`
// loads the user's full init in this process tree, which can hang on
// interactive prompts (direnv, etc.) and is host-dependent. The T7 unit test
// (`TestEmacs_UpSpawnsDaemonWhenClientErrors`) covers that path with a fake
// runner — the integration test only needs to prove that the dispatch wire
// works against a real daemon.
//
// We pass `-Q` to `emacs --daemon=...` in test setup so the daemon spins up
// quickly and reliably independent of the user's emacs config. Daemon name is
// pid-derived (`sesh-test-<pid>`) so a stray daemon never collides with the
// user's real `sesh` daemon. Cleanup runs via t.Cleanup so a test panic still
// kills the daemon.

package emacs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml/parser"

	"github.com/ijcd/sesh/internal/plugins"
)

// requireEmacs skips the test if either emacs or emacsclient is missing on
// PATH. Both are required: emacs to spawn the daemon, emacsclient to dispatch.
func requireEmacs(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("emacs"); err != nil {
		t.Skipf("emacs not on PATH: %v", err)
	}
	if _, err := exec.LookPath("emacsclient"); err != nil {
		t.Skipf("emacsclient not on PATH: %v", err)
	}
}

// spawnTestDaemon launches `emacs -Q --daemon=<name>` and polls until
// emacsclient can reach the socket (≤10s). `-Q` is critical: it skips the
// user's init file so the daemon comes up in seconds regardless of how
// heavyweight the user's emacs config is. Registers a t.Cleanup that issues
// `(kill-emacs)` — failures during cleanup are logged, not fatal, because
// the daemon may already be dead.
func spawnTestDaemon(t *testing.T, name string) {
	t.Helper()
	if err := exec.Command("emacs", "-Q", "--daemon="+name).Run(); err != nil {
		t.Fatalf("spawn emacs daemon %q: %v", name, err)
	}
	t.Cleanup(func() {
		cmd := exec.Command("emacsclient", "--socket-name="+name, "-e", "(kill-emacs)")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("cleanup kill-emacs daemon=%q: %v (output: %s)", name, err, strings.TrimSpace(string(out)))
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		cmd := exec.Command("emacsclient", "--socket-name="+name, "-e", "(emacs-version)")
		if err := cmd.Run(); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon %q never became ready within 10s", name)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// evalInDaemon runs an elisp form against the named daemon and returns its
// printed value as a trimmed string. Fails the test on emacsclient error so
// callers don't have to handle the unhappy path verbatim.
func evalInDaemon(t *testing.T, name, form string) string {
	t.Helper()
	cmd := exec.Command("emacsclient", "--socket-name="+name, "-e", form)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("emacsclient eval %q on daemon %q: %v", form, name, err)
	}
	return strings.TrimSpace(string(out))
}

// buildPluginInstance constructs a real *plugin* Instance via the public
// factory, exercising the YAML decode path so any drift in the config schema
// surfaces at test time. The `daemon:` field is set so the plugin's
// emacsclient invocations route to our per-test daemon — emacsclient honours
// the EMACS_SOCKET_NAME env var as a default, which we set below so the bare
// `emacsclient` commands the plugin runs (no --socket-name flag) target the
// uniquely-named daemon.
func buildPluginInstance(t *testing.T, env plugins.ProjectEnv, daemon string) plugins.Instance {
	t.Helper()
	src := fmt.Sprintf("daemon: %s\nhook: sesh-open-project\nclose_hook: sesh-close-project\n", daemon)
	f, err := parser.ParseBytes([]byte(src), 0)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	raw := plugins.NewRawConfig(f.Docs[0].Body)
	inst, err := New().New(env, raw)
	if err != nil {
		t.Fatalf("Plugin.New: %v", err)
	}
	t.Setenv("EMACS_SOCKET_NAME", daemon)
	return inst
}

// TestEmacs_RealDaemonOpenClose pre-spawns the daemon under a unique name
// (avoiding the user's real `sesh` daemon), pre-installs `sesh-open-project`
// and `sesh-close-project` as test hooks that record their args into a
// daemon-side global, then runs the plugin's Up + Down and reads the global
// back to verify the daemon saw the expected open and close calls.
func TestEmacs_RealDaemonOpenClose(t *testing.T) {
	requireEmacs(t)

	daemonName := fmt.Sprintf("sesh-test-%d", os.Getpid())
	spawnTestDaemon(t, daemonName)

	// Install test hooks. `setq` on a previously-unbound symbol implicitly
	// defines it dynamically — fine for a one-shot test, no `defvar` needed.
	evalInDaemon(t, daemonName, `(defun sesh-open-project (name cwd &optional files) (setq sesh-test-result (list :open name cwd files)))`)
	evalInDaemon(t, daemonName, `(defun sesh-close-project (name) (setq sesh-test-result (list :close name)))`)
	evalInDaemon(t, daemonName, `(setq sesh-test-result nil)`)

	cwd := t.TempDir()
	env := plugins.ProjectEnv{Name: "testproj", Cwd: cwd}
	inst := buildPluginInstance(t, env, daemonName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := inst.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	// Read what the open hook saw. Printed elisp list looks like
	//   (:open "testproj" "/tmp/...." nil)
	openResult := evalInDaemon(t, daemonName, `sesh-test-result`)
	if !strings.Contains(openResult, ":open") {
		t.Errorf("open result missing :open tag: %q", openResult)
	}
	if !strings.Contains(openResult, `"testproj"`) {
		t.Errorf("open result missing project name: %q", openResult)
	}
	if !strings.Contains(openResult, fmt.Sprintf("%q", cwd)) {
		t.Errorf("open result missing cwd %q: %q", cwd, openResult)
	}
	// No files passed → optional `files` arg is nil.
	if !strings.HasSuffix(openResult, "nil)") {
		t.Errorf("open result should end with nil for no files: %q", openResult)
	}

	if err := inst.Down(ctx); err != nil {
		t.Fatalf("Down: %v", err)
	}

	closeResult := evalInDaemon(t, daemonName, `sesh-test-result`)
	if !strings.Contains(closeResult, ":close") {
		t.Errorf("close result missing :close tag: %q", closeResult)
	}
	if !strings.Contains(closeResult, `"testproj"`) {
		t.Errorf("close result missing project name: %q", closeResult)
	}
}
