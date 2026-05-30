package lua

import (
	"bytes"
	osexec "os/exec"

	lua "github.com/yuin/gopher-lua"
)

// luaExec implements sesh.exec(bin, args, opts?) — synchronous, returns
// a table { stdout = "...", stderr = "...", code = <int> }.
//
// `args` may be a Lua array of strings or nil. `opts` is currently
// ignored (placeholder for future stdin/timeout/cwd) — present so the
// spike can feel the call site shape.
func luaExec(L *lua.LState) int {
	bin := L.CheckString(1)
	args := lvalueToStringSlice(L.OptTable(2, L.NewTable()))
	_ = L.OptTable(3, L.NewTable()) // opts (unused for spike)

	cmd := osexec.Command(bin, args...)
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
	result := L.NewTable()
	L.SetField(result, "stdout", lua.LString(stdout.String()))
	L.SetField(result, "stderr", lua.LString(stderr.String()))
	L.SetField(result, "code", lua.LNumber(code))
	if err != nil && code == -1 {
		L.SetField(result, "error", lua.LString(err.Error()))
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

	cmd := osexec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		result := L.NewTable()
		L.SetField(result, "error", lua.LString(err.Error()))
		L.SetField(result, "pid", lua.LNumber(-1))
		L.Push(result)
		return 1
	}
	pid := cmd.Process.Pid
	// Reap in background so we don't leak zombies. Fire-and-forget from Lua's POV.
	go func() { _ = cmd.Wait() }()
	result := L.NewTable()
	L.SetField(result, "pid", lua.LNumber(pid))
	L.Push(result)
	return 1
}

// luaPathLookup implements sesh.path_lookup(bin) — returns path string or nil.
func luaPathLookup(L *lua.LState) int {
	bin := L.CheckString(1)
	p, err := osexec.LookPath(bin)
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
