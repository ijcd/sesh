# SPIKE: Lua plugin bridge for sesh v0.5

Branch: `ijcd/spike-lua-bridge` · Status: **DONE_WITH_CONCERNS** · Tests: 339 pass (331 baseline + 8 new)

## What got built

- `go get github.com/yuin/gopher-lua@v1.1.2`
- `internal/plugins/lua/`:
  - `bridge.go` — `sesh.*` table registration; per-State registrations slot stashed in `lua.RegistryIndex`.
  - `api_exec.go` — `sesh.exec`, `sesh.exec_detach`, `sesh.path_lookup`.
  - `api_misc.go` — `sesh.wait_for`, `sesh.log.{info,warn,error}`.
  - `plugin.go` — `LuaPlugin` and `LuaInstance` implementing the v0.4 `plugins.Plugin` / `plugins.Instance` contract; YAML→Go-any→Lua-table converter.
  - `discovery.go` — embed.FS sweep + `~/.config/sesh/plugins/*.lua` sweep; takes a `Register func(plugins.Plugin) error` callback.
  - `embed/emacs.lua` — full port of the Go emacs plugin (~110 lines vs the Go original ~210 lines combined `emacs.go` + `elisp.go`).
  - `lua_test.go` — 8 tests covering: sentinel-file up/down, validate-table return, string-error return, lua-runtime-error propagation, embedded emacs DryRun + cfg-override DryRun, duplicate-register, path_lookup miss.
- `cmd/sesh/root.go` — opt-in via `SESH_USE_LUA_PLUGINS=1`; registers `emacs-lua` distinct from Go `emacs` so both coexist for A/B.

## What worked

The contract maps cleanly. `plugins.Plugin.Name()` is a captured string; `plugins.Plugin.New()` is a `cfg.Decode` + `goToLuaTable` + struct construction; `Instance.{Up,Down}` is one `CallByParam` with a small return-shape matcher. The bridge code is ~250 lines of Go for full coverage of the four entry points. The emacs port shrunk roughly in half — Lua's pattern matching and tables replaced a regex, a `quote()` helper, and a separate `elisp.go` file. The Lua `dry_run` for emacs is genuinely easier to read than its Go equivalent.

`sesh.exec` returning `{ stdout, stderr, code }` felt natural in the port (`if probe.code ~= 0 then`). `sesh.wait_for(predicate, ms)` collapsed the `waitForDaemon` Go loop into one call. `sesh.path_lookup` returning nil-or-string maps to the `if not sesh.path_lookup(...)` idiom unchanged from Lua practice. Returning the up/down error as `return nil` (success) or `return "msg"` (failure) reads correctly to a Lua author — it mirrors `pcall`'s second return slot.

## What was awkward

**`DryRun` return shape mismatch.** The proposed API in the prompt says `dry_run` returns a table of argv tables (`{ {"bin","arg"}, ... }`). The existing v0.4 SPI signature is `DryRun() ([]string, error)` — a single argv. The bridge currently flattens by taking row 1. This is a real interface break, not a translation hiccup: the Lua side wants to express "this is what I'd run", which is plural when a plugin probes + spawns + dispatches; the Go side was designed for the single-argv emacs case and would have to grow. Two options for v0.5: change `plugins.Instance.DryRun` to `[][]string`, or commit to "one argv per Instance, always". The Lua API in the prompt presumes the former.

**`RawConfig` → Lua table is a two-hop dance.** The bridge calls `cfg.Decode(&raw)` into a `var raw any`, then walks the resulting `map[string]any` / `[]any` / scalar tree into a Lua table via `goToLua`. Two awkward bits: (1) goccy/go-yaml emits some maps as `map[any]any` (non-string keys), which the converter has to handle; (2) numeric types arrive as `int` or `float64` depending on YAML formatting, and Lua doesn't distinguish, so we lose a small amount of type fidelity (acceptable). A direct YAML-AST → Lua converter would skip the middle step but requires a node-walking adapter; the two-hop cost is small enough that this is probably premature. Flag for the plan: decide whether the `RawConfig` interface grows a `DecodeTo(LValue)` method or stays Go-typed and the bridge does the work each time.

**Error marshaling.** Three error channels collapse to one Go error: (a) Lua runtime error via `error()`; (b) string return value from `up`/`down`; (c) the Go bridge code itself failing (e.g. missing field). All three are folded into `fmt.Errorf("lua plugin %q: %s: %w/%s", name, fn, ...)`. The prefix tells the user where it came from but the kind is lost. In practice that's probably fine — the engine wraps again as `apps[N] X: Up failed: ...` — but the layering means there's no programmatic way to ask "was this a Lua bug or a plugin's intentional error?". Probably OK for v0.5; revisit if someone needs to test plugin-vs-bridge errors separately.

**`validate` return shape.** Settled on table-of-strings. `{ "msg1", "msg2" }`. Easy to write in Lua; easy to map to `[]error` in Go. Considered `{ {field=..., msg=...}, ... }` (structured errors); not needed for the spike. If the engine ever wants field-level pointing for IDE integration, the shape can grow without breaking the simple form.

**`wait_for` error semantics.** If the predicate itself raises, the spike treats that as "not yet true" and keeps polling. This is wrong if the predicate has a real bug — you'd hit the timeout without ever knowing. The right behavior is probably "if pcall fails, abort and propagate". Easy fix; left as-is in spike to flag the call.

**Per-plugin vs shared LState.** The spike uses **one shared LState** across all plugins from a single LoadAll. Simple, fast (single state init at startup). Risk: a Lua plugin can mutate `sesh.*` or globals and affect another plugin loaded after it. Acceptable for the spike. Production decision: probably one LState per LuaPlugin (re-load source per state). Cost is small (~tens of µs per state).

## Performance hunches

- Bridge startup: `newState()` + `registerSeshAPI` is ~milliseconds. `LoadAll` for one embedded file (`emacs.lua`) measures sub-millisecond on M-series. Not a concern.
- Per-dispatch cost: `New` does YAML decode + tree walk to Lua table; `Up` is one `CallByParam`. The locking on `LuaPlugin.mu` serializes all calls to one plugin's State. For sesh's pattern (Up is sequential, one Apps[] entry at a time) this is fine. For hypothetical concurrent dispatch within one plugin, the mutex bottlenecks; one-state-per-plugin would too, so this is inherent to gopher-lua, not the design.
- The embedded emacs port spawns 3-4 exec calls; per-exec setup dominates the dispatch loop. Lua overhead is invisible in profile.

## Sandbox question

Standard Lua stdlib is loaded as-is. That means `os.execute`, `io.popen`, `os.exit`, `os.remove`, `io.open` are all available to plugin authors. For a user-installed plugin from a trusted source this is fine — and probably necessary, since plugins WILL want to write files and read configs. For untrusted plugins this is a remote-code-execution surface. The v0.5 plan needs an explicit stance: **trust model is "plugins are code the user installed"** (same as VS Code extensions, neovim plugins, emacs packages). If that's accepted, no sandboxing. If not, gopher-lua's `OpenLibsFunc` lets you load only `string`, `math`, `table`, `pcall` etc., omitting `os` and `io` entirely — straightforward to wire.

One subtle gotcha: `dofile`, `loadfile`, `require` in the stdlib will look at filesystem paths set by Lua's `package.path`, which gopher-lua initializes from env vars. A malicious plugin could `require("/etc/passwd")` etc. — but that's not RCE, just file read. Same trust model applies.

## Discovery question

Embedded-first then user-dir, both sorted by filename, deduplicated by registered name with user-dir winning. Felt natural. The "user-dir shadows embedded" semantic was easy to implement (last-write-wins on the registrations slice before any `Register()` call). Two specific findings:

- **Override needs to be on the registered name, not the filename.** The user-dir file can be called `my-emacs.lua` and still register as `"emacs-lua"` to shadow the embedded one. That's the right semantic — filename is just a sort key.
- **No version pinning, no dependencies.** The spike doesn't try to express "plugin A depends on plugin B". Each plugin is a closed unit. Fine for v0.5; if plugin ecosystems develop, revisit.

## API additions made beyond the prompt

- `sesh.exec` returns an `error` field in the result table on spawn failure (`code = -1`). The prompt's shape was `{ stdout, stderr, code }`; the addition is small but matters because `code` alone can't distinguish "ran and exited 1" from "didn't run".
- `sesh.exec_detach` returns `error` + `pid = -1` on spawn failure for the same reason.
- A goroutine reaps detached processes to avoid zombies. Lua-invisible, but documented in code.

No `applescript` or `state_path` helpers were added — they didn't come up in the port. Flag them as v0.5 design questions.

## The big question — would v0.5 land on this design?

**Mostly yes, with two changes first:**

1. **Promote `DryRun` to `[][]string`.** The Lua `dry_run` returning a list of argvs is the right shape for plugins that do multiple shell calls. The Go side should match. This is a one-line interface change with no migration burden (single Go plugin is emacs, and it'd just wrap its single argv in an outer slice). It also makes `dry_run` actually preview the spawn-fallback path that the current Go emacs `DryRun` deliberately skips.

2. **Decide LState ownership.** Either (a) one-per-plugin from rehydrate-on-register, or (b) explicitly document shared-state as the contract and lean into it. The spike's accidental "one shared" is a non-decision and should be made explicit before v0.5 promises stability.

Everything else feels right. The API surface is small and the friction points map to design calls, not bridge bugs. The emacs port shrinking by ~50% is encouraging — that's where the user-facing win lives.

### Smaller calls to make in the plan

- `wait_for`: pcall the predicate; propagate on error.
- Per-plugin LState: pick one, document it.
- Trust model: state explicitly. Default no-sandbox.
- `RawConfig` → Lua: keep the two-hop converter for now; revisit only if YAML type fidelity bites.
- Embedded plugins: confirm the embed.FS pattern is acceptable for shipping default plugins (vs distributing them as separate files).

## Files created

- `internal/plugins/lua/bridge.go`
- `internal/plugins/lua/api_exec.go`
- `internal/plugins/lua/api_misc.go`
- `internal/plugins/lua/plugin.go`
- `internal/plugins/lua/discovery.go`
- `internal/plugins/lua/embed/emacs.lua`
- `internal/plugins/lua/lua_test.go`
- `docs/superpowers/specs/SPIKE-NOTES.md` (this file)

## Files modified

- `go.mod`, `go.sum` (added gopher-lua)
- `cmd/sesh/root.go` (opt-in lua loader behind `SESH_USE_LUA_PLUGINS`)
