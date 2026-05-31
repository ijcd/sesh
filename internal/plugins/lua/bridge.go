// Package lua wraps gopher-lua to expose a `sesh.*` API to user-authored
// Lua plugins and adapt them to the v0.4 plugins.Plugin / plugins.Instance
// contract.
//
// Design call: one *lua.LState per registered plugin (LuaPlugin). Cheap
// (~tens of µs per State), avoids cross-plugin mutation of `sesh.*` table
// or globals, and lets each plugin's `up`/`down` keep upvalue closures
// without surprises. Discovery is two-pass: first pass runs each source
// in a throwaway state with a stub sesh.register that records (name →
// source-bytes); second pass evaluates each source again in a fresh
// LState with the real API and binds the captured plugin table to a
// LuaPlugin.
package lua

import (
	"context"
	"fmt"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// ctxByLState lets the sesh.* API functions (luaExec, luaExecDetach,
// luaApplescript) retrieve the caller's ctx without touching the Lua
// API surface. LuaInstance.Up/Down bracket their work with
// setStateCtx / clearStateCtx so an in-flight exec inherits the
// dispatch's deadline / cancellation.
//
// Lookups fall back to context.Background() — e.g., when the bridge is
// driven outside an Up/Down path (today: only the discovery pass, which
// uses a separate stub state and never reaches these seams).
var (
	ctxMu       sync.RWMutex
	ctxByLState = map[*lua.LState]context.Context{}
)

func setStateCtx(L *lua.LState, ctx context.Context) {
	ctxMu.Lock()
	defer ctxMu.Unlock()
	ctxByLState[L] = ctx
}

func clearStateCtx(L *lua.LState) {
	ctxMu.Lock()
	defer ctxMu.Unlock()
	delete(ctxByLState, L)
}

func stateCtx(L *lua.LState) context.Context {
	ctxMu.RLock()
	defer ctxMu.RUnlock()
	if c, ok := ctxByLState[L]; ok {
		return c
	}
	return context.Background()
}

// newState constructs a fresh gopher-lua state with the `sesh.*` API
// registered. The standard Lua libraries are loaded as-is in v0.5;
// sandboxing (restricting os.execute, io.popen, etc.) is a design
// question deliberately deferred.
func newState() *lua.LState {
	L := lua.NewState()
	registerSeshAPI(L)
	return L
}

// registerSeshAPI installs the `sesh` global table and its sub-tables.
func registerSeshAPI(L *lua.LState) {
	sesh := L.NewTable()

	// sesh.register(name, table) — the discovery entry point. Each plugin
	// file calls this once; the bridge captures the (name, table) pair
	// into the registry field on the user-data slot we stash below.
	L.SetField(sesh, "register", L.NewFunction(luaRegister))

	// sesh.exec / exec_detach / path_lookup / file_exists
	L.SetField(sesh, "exec", L.NewFunction(luaExec))
	L.SetField(sesh, "exec_detach", L.NewFunction(luaExecDetach))
	L.SetField(sesh, "path_lookup", L.NewFunction(luaPathLookup))
	L.SetField(sesh, "file_exists", L.NewFunction(luaFileExists))

	// sesh.wait_for
	L.SetField(sesh, "wait_for", L.NewFunction(luaWaitFor))

	// sesh.applescript
	L.SetField(sesh, "applescript", L.NewFunction(luaApplescript))

	// sesh.log.{info,warn,error}
	logTbl := L.NewTable()
	L.SetField(logTbl, "info", L.NewFunction(luaLogInfo))
	L.SetField(logTbl, "warn", L.NewFunction(luaLogWarn))
	L.SetField(logTbl, "error", L.NewFunction(luaLogError))
	L.SetField(sesh, "log", logTbl)

	L.SetGlobal("sesh", sesh)
}

// registryKey is the Lua-registry slot we stash the *registrations map in
// so sesh.register can append to it. Using the Lua registry (an opaque
// per-State KV table) keeps us from leaking the map onto user-visible
// globals.
const registryKey = "__sesh_registrations"

// registrations is the running list of plugins registered from Lua during
// a discovery pass. Order = source order (deterministic by file walk).
type registrations struct {
	entries []registration
}

type registration struct {
	name string
	tbl  *lua.LTable // the table passed to sesh.register; holds up/down/...
}

// setRegistrations stashes r in L's registry so luaRegister can find it.
func setRegistrations(L *lua.LState, r *registrations) {
	ud := L.NewUserData()
	ud.Value = r
	L.SetField(L.Get(lua.RegistryIndex), registryKey, ud)
}

// getRegistrations returns the current *registrations, or nil if unset.
func getRegistrations(L *lua.LState) *registrations {
	v := L.GetField(L.Get(lua.RegistryIndex), registryKey)
	ud, ok := v.(*lua.LUserData)
	if !ok {
		return nil
	}
	r, _ := ud.Value.(*registrations)
	return r
}

// luaRegister implements sesh.register(name, tbl). Returns nothing.
func luaRegister(L *lua.LState) int {
	name := L.CheckString(1)
	tbl := L.CheckTable(2)
	r := getRegistrations(L)
	if r == nil {
		L.RaiseError("sesh.register called outside discovery (no registrations slot)")
		return 0
	}
	for _, e := range r.entries {
		if e.name == name {
			L.RaiseError("sesh.register: duplicate name %q", name)
			return 0
		}
	}
	r.entries = append(r.entries, registration{name: name, tbl: tbl})
	return 0
}

// callPluginFn invokes the named Lua function on tbl with the env table
// and cfg table as args. Returns the single Lua return value (any) and
// any runtime error.
//
// Return-value convention (per plugin API): up/down return a string error
// or nil for success.
func callPluginFn(L *lua.LState, tbl *lua.LTable, fnName string, env, cfg *lua.LTable) (lua.LValue, error) {
	fn := L.GetField(tbl, fnName)
	if fn == lua.LNil {
		return lua.LNil, fmt.Errorf("plugin missing %q", fnName)
	}
	if fn.Type() != lua.LTFunction {
		return lua.LNil, fmt.Errorf("plugin field %q is %s, not function", fnName, fn.Type())
	}
	if err := L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, env, cfg); err != nil {
		return lua.LNil, err
	}
	ret := L.Get(-1)
	L.Pop(1)
	return ret, nil
}
