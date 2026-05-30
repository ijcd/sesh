package lua

import (
	"fmt"
	"os"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// luaWaitFor implements sesh.wait_for(predicate, timeout_ms). Polls the
// predicate every 100ms; returns true on truthy, false on timeout.
// Predicate may itself error (via Lua error()) — the spike treats that
// as "not yet true" and keeps polling. That choice is up for debate;
// flagged in SPIKE-NOTES.
func luaWaitFor(L *lua.LState) int {
	fn := L.CheckFunction(1)
	timeoutMs := L.OptInt(2, 5000)
	pollInterval := 100 * time.Millisecond
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

	for {
		if err := L.CallByParam(lua.P{
			Fn:      fn,
			NRet:    1,
			Protect: true,
		}); err == nil {
			ret := L.Get(-1)
			L.Pop(1)
			if lua.LVAsBool(ret) {
				L.Push(lua.LTrue)
				return 1
			}
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
