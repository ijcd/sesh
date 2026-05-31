package lua

import (
	"bytes"
	osexec "os/exec"
	"runtime"

	lua "github.com/yuin/gopher-lua"
)

// execOsascript is the dispatch seam for sesh.applescript. Tests override
// to capture argv without spawning osascript. Default impl shells out via
// `osascript -e <script>` and returns (combined-output, exitcode, error).
//
// Multi-line scripts are supported: osascript accepts a single -e with
// embedded newlines.
var execOsascript = func(args ...string) ([]byte, int, error) {
	cmd := osexec.Command("osascript", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*osexec.ExitError); ok {
			return out.Bytes(), ee.ExitCode(), err
		}
		return out.Bytes(), -1, err
	}
	return out.Bytes(), 0, nil
}

// luaApplescript implements sesh.applescript(script) — runs the given
// AppleScript via osascript and returns { stdout = "...", code = <int>,
// error = "..." }. On non-darwin hosts, returns { stdout = "", code = -1,
// error = "osascript not available on this platform" } so cross-platform
// plugins can branch on the error string.
func luaApplescript(L *lua.LState) int {
	script := L.CheckString(1)

	result := L.NewTable()
	if runtime.GOOS != "darwin" {
		L.SetField(result, "stdout", lua.LString(""))
		L.SetField(result, "code", lua.LNumber(-1))
		L.SetField(result, "error", lua.LString("osascript not available on this platform"))
		L.Push(result)
		return 1
	}

	out, code, err := execOsascript("-e", script)
	L.SetField(result, "stdout", lua.LString(string(out)))
	L.SetField(result, "code", lua.LNumber(code))
	if err != nil && code == -1 {
		L.SetField(result, "error", lua.LString(err.Error()))
	} else {
		L.SetField(result, "error", lua.LString(""))
	}
	L.Push(result)
	return 1
}
