package lua

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ijcd/sesh/internal/plugins"
	lua "github.com/yuin/gopher-lua"
)

// LuaPlugin adapts a Lua-side plugin table (the second arg to
// sesh.register) to the plugins.Plugin contract. One LuaPlugin per
// registered name; each owns its own *lua.LState so plugins cannot
// mutate each other's globals or sesh.* tables. Discovery rehydrates
// every registered name into a dedicated LState by re-evaluating the
// source bytes; the original discovery-pass state is discarded.
type LuaPlugin struct {
	name string
	L    *lua.LState
	tbl  *lua.LTable // table with up/down/validate/dry_run
	mu   sync.Mutex  // gopher-lua LState is not goroutine-safe
}

// Name reports the plugin's registry key.
func (p *LuaPlugin) Name() string { return p.name }

// New decodes cfg into a Lua table, builds the env table, and returns a
// LuaInstance bound to both. The instance reuses p.L; serializing access
// is handled by p.mu inside each Instance method.
func (p *LuaPlugin) New(env plugins.ProjectEnv, cfg plugins.RawConfig) (plugins.Instance, error) {
	// Decode RawConfig into a generic Go map first, then convert to a
	// Lua table. This is the awkward step: YAML→Go-map→Lua-table is two
	// hops. Direct YAML→Lua would skip the middle step but requires a
	// node-walking adapter. Spike accepts the two-hop cost.
	var raw any
	if err := cfg.Decode(&raw); err != nil {
		return nil, fmt.Errorf("lua plugin %q: decode config: %w", p.name, err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	envTbl := p.L.NewTable()
	p.L.SetField(envTbl, "name", lua.LString(env.Name))
	p.L.SetField(envTbl, "cwd", lua.LString(env.Cwd))

	cfgTbl := goToLuaTable(p.L, raw)

	return &LuaInstance{
		plugin: p,
		env:    envTbl,
		cfg:    cfgTbl,
	}, nil
}

// LuaInstance is one (plugin, env, cfg) triple.
type LuaInstance struct {
	plugin *LuaPlugin
	env    *lua.LTable
	cfg    *lua.LTable
}

// Up calls the Lua-side up(env, cfg). Convention: returns string=error
// or nil=success. Lua-side errors (via error()) propagate as Go errors.
// ctx is stashed on the LState so sesh.exec / sesh.exec_detach /
// sesh.applescript can pick it up and honor cancellation; cleared on
// return so leaking the ctx into a later Validate/DryRun is impossible.
func (i *LuaInstance) Up(ctx context.Context) error {
	return i.callWithCtx(ctx, "up")
}

// Down mirrors Up.
func (i *LuaInstance) Down(ctx context.Context) error {
	return i.callWithCtx(ctx, "down")
}

// Validate calls the Lua-side validate(env, cfg). Convention: returns a
// table of error strings (may be empty). Absence of the validate field
// is OK — no errors.
func (i *LuaInstance) Validate() []error {
	i.plugin.mu.Lock()
	defer i.plugin.mu.Unlock()

	fn := i.plugin.L.GetField(i.plugin.tbl, "validate")
	if fn == lua.LNil {
		return nil
	}
	if fn.Type() != lua.LTFunction {
		return []error{fmt.Errorf("lua plugin %q: validate is %s, not function", i.plugin.name, fn.Type())}
	}
	if err := i.plugin.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, i.env, i.cfg); err != nil {
		return []error{fmt.Errorf("lua plugin %q: validate: %w", i.plugin.name, err)}
	}
	ret := i.plugin.L.Get(-1)
	i.plugin.L.Pop(1)
	tbl, ok := ret.(*lua.LTable)
	if !ok || tbl == nil {
		return nil
	}
	var errs []error
	n := tbl.Len()
	for k := 1; k <= n; k++ {
		v := tbl.RawGetInt(k)
		if v == lua.LNil {
			break
		}
		errs = append(errs, errors.New(v.String()))
	}
	return errs
}

// DryRun calls the Lua-side dry_run(env, cfg). Convention: returns a
// table of argv tables: { {"bin","a","b"}, {"bin2","a"}, ... }. Each
// inner table is one shell call the plugin would invoke; the outer
// order matches Up's call sequence. An empty outer table yields a
// non-nil empty [][]string{} (no rows to print).
func (i *LuaInstance) DryRun() ([][]string, error) {
	i.plugin.mu.Lock()
	defer i.plugin.mu.Unlock()

	fn := i.plugin.L.GetField(i.plugin.tbl, "dry_run")
	if fn == lua.LNil {
		return nil, nil
	}
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("lua plugin %q: dry_run is %s, not function", i.plugin.name, fn.Type())
	}
	if err := i.plugin.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, i.env, i.cfg); err != nil {
		return nil, fmt.Errorf("lua plugin %q: dry_run: %w", i.plugin.name, err)
	}
	ret := i.plugin.L.Get(-1)
	i.plugin.L.Pop(1)
	tbl, ok := ret.(*lua.LTable)
	if !ok || tbl == nil {
		return nil, nil
	}
	n := tbl.Len()
	rows := make([][]string, 0, n)
	for k := 1; k <= n; k++ {
		v := tbl.RawGetInt(k)
		row, ok := v.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("lua plugin %q: dry_run row %d is %s, not table", i.plugin.name, k, v.Type())
		}
		rows = append(rows, lvalueToStringSlice(row))
	}
	return rows, nil
}

// callWithCtx is the shared Up/Down implementation. Lua convention:
// return nil=success, string=error message. Lua runtime errors (via
// error()) propagate as Go errors too. ctx is stashed on the LState
// for the duration of the call so sesh.exec / sesh.exec_detach /
// sesh.applescript can honor cancellation.
func (i *LuaInstance) callWithCtx(ctx context.Context, fnName string) error {
	i.plugin.mu.Lock()
	defer i.plugin.mu.Unlock()

	setStateCtx(i.plugin.L, ctx)
	defer clearStateCtx(i.plugin.L)

	ret, err := callPluginFn(i.plugin.L, i.plugin.tbl, fnName, i.env, i.cfg)
	if err != nil {
		return fmt.Errorf("lua plugin %q: %s: %w", i.plugin.name, fnName, err)
	}
	if ret == lua.LNil {
		return nil
	}
	if s, ok := ret.(lua.LString); ok {
		if string(s) == "" {
			return nil
		}
		return fmt.Errorf("lua plugin %q: %s: %s", i.plugin.name, fnName, string(s))
	}
	return fmt.Errorf("lua plugin %q: %s: unexpected return %s", i.plugin.name, fnName, ret.Type())
}

// goToLuaTable converts a Go value (typically the YAML-decoded map) to
// a Lua table. Handles map[string]any, []any, primitives. Anything
// else is converted via fmt.Sprintf("%v"). Mirrors what a small
// json.Marshal-style converter would do.
func goToLuaTable(L *lua.LState, v any) *lua.LTable {
	tbl := L.NewTable()
	if v == nil {
		return tbl
	}
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			L.SetField(tbl, k, goToLua(L, val))
		}
	case map[any]any:
		// goccy/go-yaml may yield this for non-string keys; coerce keys.
		for k, val := range x {
			L.SetField(tbl, fmt.Sprintf("%v", k), goToLua(L, val))
		}
	case []any:
		for i, val := range x {
			tbl.RawSetInt(i+1, goToLua(L, val))
		}
	default:
		// Scalar at top level — wrap as { value = scalar } so the plugin
		// can still index it. Shouldn't normally happen for apps-block
		// config.
		L.SetField(tbl, "value", goToLua(L, x))
	}
	return tbl
}

func goToLua(L *lua.LState, v any) lua.LValue {
	// Note: lua.LNumber is float64 underneath; uint64 values exceeding
	// 2^53 lose precision. That's an inherent Lua limitation; we cover
	// every stdlib numeric type but the precision ceiling is unavoidable.
	switch x := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(x)
	case string:
		return lua.LString(x)
	case int:
		return lua.LNumber(x)
	case int8:
		return lua.LNumber(x)
	case int16:
		return lua.LNumber(x)
	case int32:
		return lua.LNumber(x)
	case int64:
		return lua.LNumber(x)
	case uint:
		return lua.LNumber(x)
	case uint8:
		return lua.LNumber(x)
	case uint16:
		return lua.LNumber(x)
	case uint32:
		return lua.LNumber(x)
	case uint64:
		return lua.LNumber(x)
	case float32:
		return lua.LNumber(x)
	case float64:
		return lua.LNumber(x)
	case []any:
		t := L.NewTable()
		for i, e := range x {
			t.RawSetInt(i+1, goToLua(L, e))
		}
		return t
	case map[string]any:
		return goToLuaTable(L, x)
	case map[any]any:
		return goToLuaTable(L, x)
	default:
		return lua.LString(fmt.Sprintf("%v", x))
	}
}

// Compile-time assertions matching the v0.4 SPI.
var (
	_ plugins.Plugin   = (*LuaPlugin)(nil)
	_ plugins.Instance = (*LuaInstance)(nil)
)
