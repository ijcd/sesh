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
// registered name; its LState is the same one used during discovery,
// kept alive for the lifetime of the process.
//
// Spike design call: a single shared LState across all LuaPlugins
// loaded in one discovery pass. Cheap and simple. The downside is that
// any Lua plugin can in principle mutate `sesh` (or any global) and
// affect another plugin loaded after. Acceptable for the spike; the
// production design likely wants one LState per plugin (rehydrated from
// the original source) for isolation.
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
func (i *LuaInstance) Up(ctx context.Context) error {
	return i.call("up")
}

// Down mirrors Up.
func (i *LuaInstance) Down(ctx context.Context) error {
	return i.call("down")
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
// table of argv tables: { {"bin","a","b"}, {"bin2","a"}, ... }.
// To fit the existing plugins.Instance.DryRun() ([]string, error)
// signature (which is a SINGLE argv), the spike flattens the first
// returned argv tuple. This mismatch is a load-bearing finding — see
// SPIKE-NOTES.
func (i *LuaInstance) DryRun() ([]string, error) {
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
	// Take first row (argv); future API might return all rows.
	if tbl.Len() == 0 {
		return nil, nil
	}
	firstRow, ok := tbl.RawGetInt(1).(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("lua plugin %q: dry_run row 1 is %s, not table", i.plugin.name, tbl.RawGetInt(1).Type())
	}
	return lvalueToStringSlice(firstRow), nil
}

// call is the shared Up/Down implementation. Lua convention: return
// nil=success, string=error message. Lua runtime errors (via error())
// propagate as Go errors too.
func (i *LuaInstance) call(fnName string) error {
	i.plugin.mu.Lock()
	defer i.plugin.mu.Unlock()

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
	switch x := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(x)
	case string:
		return lua.LString(x)
	case int:
		return lua.LNumber(x)
	case int64:
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
