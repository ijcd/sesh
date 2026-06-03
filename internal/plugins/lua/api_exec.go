package lua

import (
	"bytes"
	"context"
	"fmt"
	"os"
	osexec "os/exec"

	lua "github.com/yuin/gopher-lua"
)

// execResult is the structured return from execRunner / execDetachRunner.
// Mirrors what gets surfaced to Lua as the result table.
type execResult struct {
	Stdout string
	Stderr string
	Code   int
	Err    error // non-nil when the process couldn't be started
}

// execRunner is the dispatch seam for sesh.exec. Tests override to capture
// argv and return scripted results without spawning. Default impl shells
// out via os/exec with the caller's ctx so dispatch deadlines / SIGINT
// can cancel an in-flight subprocess.
//
// Pattern mirrors execOsascript in api_applescript.go.
var execRunner = func(ctx context.Context, bin string, args []string) execResult {
	cmd := osexec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*osexec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return execResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Code:   code,
		Err:    err,
	}
}

// execDetachRunner is the dispatch seam for sesh.exec_detach. Tests
// override to capture argv. Default impl spawns and reaps in the
// background. ctx governs the spawn only — once Start succeeds, the
// child outlives the ctx (that's the point of "detach": Firefox spawned
// by `sesh up` must survive `sesh up` returning). We honor an
// already-cancelled ctx at entry, then drop it.
var execDetachRunner = func(ctx context.Context, bin string, args []string) (pid int, err error) {
	if err := ctx.Err(); err != nil {
		return -1, err
	}
	cmd := osexec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	pid = cmd.Process.Pid
	// Reap in background so we don't leak zombies. Fire-and-forget from
	// Lua's POV, but surface non-zero exits to stderr — Firefox crashing
	// immediately after launch would otherwise leave Up returning success
	// with no diagnostic.
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "sesh: detached %s (pid %d) exited with error: %v\n", bin, pid, err)
		}
	}()
	return pid, nil
}

// pathLookup is the dispatch seam for sesh.path_lookup. Tests override
// to simulate missing binaries.
var pathLookup = func(bin string) (string, error) {
	return osexec.LookPath(bin)
}

// luaExec implements sesh.exec(bin, args, opts?) — synchronous, returns
// a table { stdout = "...", stderr = "...", code = <int> }.
//
// `args` may be a Lua array of strings or nil. `opts` is currently
// ignored (placeholder for future stdin/timeout/cwd) — present so
// callers can shape the call site.
func luaExec(L *lua.LState) int {
	bin := L.CheckString(1)
	args := lvalueToStringSlice(L.OptTable(2, L.NewTable()))
	_ = L.OptTable(3, L.NewTable()) // opts (unused in v0.5)

	r := execRunner(stateCtx(L), bin, args)
	result := L.NewTable()
	L.SetField(result, "stdout", lua.LString(r.Stdout))
	L.SetField(result, "stderr", lua.LString(r.Stderr))
	L.SetField(result, "code", lua.LNumber(r.Code))
	if r.Err != nil && r.Code == -1 {
		L.SetField(result, "error", lua.LString(r.Err.Error()))
	}
	L.Push(result)
	return 1
}

// luaExecDetach implements sesh.exec_detach(bin, args, opts?) — spawns
// and returns { pid = <int> } without waiting.
func luaExecDetach(L *lua.LState) int {
	bin := L.CheckString(1)
	args := lvalueToStringSlice(L.OptTable(2, L.NewTable()))
	_ = L.OptTable(3, L.NewTable())

	pid, err := execDetachRunner(stateCtx(L), bin, args)
	result := L.NewTable()
	if err != nil {
		L.SetField(result, "error", lua.LString(err.Error()))
		L.SetField(result, "pid", lua.LNumber(-1))
		L.Push(result)
		return 1
	}
	L.SetField(result, "pid", lua.LNumber(pid))
	L.Push(result)
	return 1
}

// luaPathLookup implements sesh.path_lookup(bin) — returns path string or nil.
func luaPathLookup(L *lua.LState) int {
	bin := L.CheckString(1)
	p, err := pathLookup(bin)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(p))
	return 1
}

// lvalueToStringSlice converts a Lua array-style table to a []string,
// stopping at the first nil. Non-string entries are coerced via tostring.
func lvalueToStringSlice(t *lua.LTable) []string {
	var out []string
	n := t.Len()
	for i := 1; i <= n; i++ {
		v := t.RawGetInt(i)
		if v == lua.LNil {
			break
		}
		out = append(out, v.String())
	}
	return out
}
