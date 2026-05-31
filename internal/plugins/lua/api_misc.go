package lua

import (
	"fmt"
	"os"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// fileExistsFunc is the dispatch seam for sesh.file_exists. Tests override
// to simulate present/missing files without touching the filesystem.
// Default impl is os.Stat — error means "doesn't exist (or unreadable)",
// which we treat as false. Mirrors the seam pattern from execRunner /
// pathLookup so plugin tests can script fallback-path detection.
var fileExistsFunc = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// luaFileExists implements sesh.file_exists(path) -> bool. Useful for
// plugins that probe a well-known fallback path (e.g., an .app bundle)
// when PATH lookup misses.
func luaFileExists(L *lua.LState) int {
	path := L.CheckString(1)
	L.Push(lua.LBool(fileExistsFunc(path)))
	return 1
}

// luaWaitFor implements sesh.wait_for(predicate, timeout_ms). Polls the
// predicate every 100ms; returns true on truthy, false on timeout.
// Predicate errors (via Lua error()) are re-raised as Lua errors via
// L.RaiseError — surface to the calling Up/Down so a buggy predicate
// fails visibly rather than silently spinning until timeout.
func luaWaitFor(L *lua.LState) int {
	fn := L.CheckFunction(1)
	timeoutMs := L.OptInt(2, 5000)
	pollInterval := 100 * time.Millisecond
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

	for {
		err := L.CallByParam(lua.P{
			Fn:      fn,
			NRet:    1,
			Protect: true,
		})
		if err != nil {
			// Re-raise as Lua error; CallByParam in the caller will see it
			// (Option B from the plan: traceback rather than (false, errstr)).
			L.RaiseError("sesh.wait_for: predicate error: %s", err.Error())
			return 0
		}
		ret := L.Get(-1)
		L.Pop(1)
		if lua.LVAsBool(ret) {
			L.Push(lua.LTrue)
			return 1
		}
		if time.Now().After(deadline) {
			L.Push(lua.LFalse)
			return 1
		}
		time.Sleep(pollInterval)
	}
}

// luaLogInfo writes [sesh-lua INFO] <msg> to stderr.
func luaLogInfo(L *lua.LState) int  { return logAt(L, "INFO") }
func luaLogWarn(L *lua.LState) int  { return logAt(L, "WARN") }
func luaLogError(L *lua.LState) int { return logAt(L, "ERROR") }

func logAt(L *lua.LState, level string) int {
	msg := L.CheckString(1)
	fmt.Fprintf(os.Stderr, "[sesh-lua %s] %s\n", level, msg)
	return 0
}
