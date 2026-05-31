//go:build integration_lua

// Real-emacs integration test for the Lua emacs plugin. Gated
// `integration_lua` so plain `go test ./...` keeps skipping it (no emacs
// dependency on CI).
//
// Coverage: pre-spawn a uniquely-named emacs daemon, install test hooks
// that record their argv into a daemon-side global, then run plugin
// Up/Down against the daemon via the Lua bridge's public LoadAll path
// (exercising the embedded emacs.lua + YAML→Lua-table config decode).
// Asserts the daemon-side global was mutated to the expected
// (:open …) / (:close …) shape.
//
// We pass `-Q` to `emacs --daemon=…` in test setup so the daemon spins
// up quickly and reliably independent of the user's emacs config. Daemon
// name is pid-derived (`sesh-test-<pid>`) so a stray daemon never
// collides with the user's real `sesh` daemon. Cleanup runs via
// t.Cleanup so a test panic still kills the daemon.
//
// Unlike the v0.4 Go-emacs integration test, this one does NOT need the
// EMACS_SOCKET_NAME env workaround: the Lua plugin always passes
// --socket-name=<daemon> on every emacsclient invocation, so the daemon
// override in cfg is honored end-to-end.

package lua

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
// emacsclient can reach the socket (≤10s). `-Q` is critical: it skips
// the user's init file so the daemon comes up in seconds regardless of
// how heavyweight the user's emacs config is. Registers a t.Cleanup
// that issues `(kill-emacs)` — failures during cleanup are logged, not
// fatal, because the daemon may already be dead.
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

// evalInDaemon runs an elisp form against the named daemon and returns
// its printed value as a trimmed string. Fails the test on emacsclient
// error so callers don't have to handle the unhappy path verbatim.
func evalInDaemon(t *testing.T, name, form string) string {
	t.Helper()
	cmd := exec.Command("emacsclient", "--socket-name="+name, "-e", form)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("emacsclient eval %q on daemon %q: %v", form, name, err)
	}
	return strings.TrimSpace(string(out))
}

// buildLuaEmacsInstance loads the embedded emacs.lua via the public
// LoadAll path, builds a YAML config that points at our test daemon,
// and returns the resulting Instance. Exercises YAML→Lua-table decode
// so config-schema drift surfaces at test time.
func buildLuaEmacsInstance(t *testing.T, env plugins.ProjectEnv, daemon string) plugins.Instance {
	t.Helper()
	reg := plugins.NewRegistry()
	if _, err := LoadAll(reg.Register); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	p, ok := reg.Get("emacs")
	if !ok {
		t.Fatal("emacs plugin not registered")
	}
	src := fmt.Sprintf("daemon: %s\nhook: sesh-open-project\nclose_hook: sesh-close-project\n", daemon)
	f, err := parser.ParseBytes([]byte(src), 0)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	raw := plugins.NewRawConfig(f.Docs[0].Body)
	inst, err := p.New(env, raw)
	if err != nil {
		t.Fatalf("Plugin.New: %v", err)
	}
	return inst
}

// TestEmacsLua_RealDaemonOpenClose pre-spawns the daemon under a unique
// name (avoiding the user's real `sesh` daemon), pre-installs
// `sesh-open-project` and `sesh-close-project` as test hooks that
// record their args into a daemon-side global, then runs the plugin's
// Up + Down and reads the global back to verify the daemon saw the
// expected open and close calls.
func TestEmacsLua_RealDaemonOpenClose(t *testing.T) {
	requireEmacs(t)

	// Defensive: clear any inherited EMACS_SOCKET_NAME so it can't paper
	// over a bug where the plugin fails to pass --socket-name=<daemon>.
	t.Setenv("EMACS_SOCKET_NAME", "")

	daemonName := fmt.Sprintf("sesh-test-%d", os.Getpid())
	spawnTestDaemon(t, daemonName)

	// Install test hooks. `setq` on a previously-unbound symbol implicitly
	// defines it dynamically — fine for a one-shot test, no `defvar` needed.
	evalInDaemon(t, daemonName, `(defun sesh-open-project (name cwd &optional files) (setq sesh-test-result (list :open name cwd files)))`)
	evalInDaemon(t, daemonName, `(defun sesh-close-project (name) (setq sesh-test-result (list :close name)))`)
	evalInDaemon(t, daemonName, `(setq sesh-test-result nil)`)

	cwd := t.TempDir()
	env := plugins.ProjectEnv{Name: "testproj", Cwd: cwd}
	inst := buildLuaEmacsInstance(t, env, daemonName)

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
