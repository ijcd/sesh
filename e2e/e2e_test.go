//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runSesh(t *testing.T, env map[string]string, args ...string) (string, error) {
	t.Helper()
	bin, err := exec.LookPath("./sesh")
	if err != nil {
		bin = filepath.Join("..", "sesh")
	}
	cmd := exec.Command(bin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	err = cmd.Run()
	return out.String(), err
}

func TestSmoke_NewLsValidateDebug(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}
	cfg := t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": cfg}

	if out, err := runSesh(t, env, "new", "demo"); err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	if out, err := runSesh(t, env, "ls"); err != nil || !strings.Contains(out, "demo") {
		t.Fatalf("ls did not list demo: err=%v out=%s", err, out)
	}
	if out, err := runSesh(t, env, "validate", "demo"); err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
	if out, err := runSesh(t, env, "debug", "demo"); err != nil {
		t.Fatalf("debug: %v\n%s", err, out)
	}
}

func TestSmoke_KittyValidateAndDebug(t *testing.T) {
	if _, err := exec.LookPath("kitten"); err != nil {
		t.Skip("kitten not on PATH")
	}
	cfg := t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": cfg}

	// sesh new <name> creates a tmux project by default; manually craft a kitty one
	projDir := filepath.Join(cfg, "sesh", "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	kittyYaml := `driver: kitty
cwd: /tmp
tabs:
  - title: shell
  - title: dev
    cmd: echo dev
`
	if err := os.WriteFile(filepath.Join(projDir, "ktest.yml"), []byte(kittyYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := runSesh(t, env, "validate", "ktest"); err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
	if out, err := runSesh(t, env, "debug", "ktest"); err != nil {
		t.Fatalf("debug: %v\n%s", err, out)
	}
}
