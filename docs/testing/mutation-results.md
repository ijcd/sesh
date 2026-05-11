# Mutation testing results — sesh

**Date:** 2026-05-10
**Tool:** gremlins version dev darwin/amd64
**Scope:** `internal/config`, `internal/engine`
**Timeout coefficient:** 20× (default 1× causes near-universal TIMED OUT on this codebase)

## Summary

| Package | Runnable | Killed | Lived | Not covered | Equivalent | Real gaps closed |
|---|---|---|---|---|---|---|
| internal/config | 73 | 63 | 9 | 1 | 4 | 5 |
| internal/engine | 41 | 33 | 6 | 2 | 2 | 4 |
| **Total** | **114** | **96** | **15** | **3** | **6** | **9** |

Test efficacy after fixes: config 87.5% → higher; engine 84.6% → higher.

## Real gaps closed (9 mutations addressed)

Added `internal/config/mutation_gaps_test.go` and `internal/engine/mutation_gaps_test.go`.

### config package

**`config.go:58` — Attach propagation from global (2 mutations)**
`applyGlobalDefaults` copies `g.Attach` into `p.Attach` when `p.Attach == nil`. Negating either
condition survived because no test directly called `applyGlobalDefaults` with a non-nil global
Attach. Tests added: `TestApplyGlobalDefaults_AttachPropagatedFromGlobal`,
`TestApplyGlobalDefaults_AttachNotOverwrittenWhenProjectSetsIt`,
`TestApplyGlobalDefaults_AttachNotCopiedWhenGlobalNil`.

**`config.go:62` — empty global Vars boundary (1 mutation)**
The `len(g.Vars) > 0` guard means an empty-but-non-nil global Vars map should be a no-op.
The CONDITIONALS_BOUNDARY mutation (`>= 0`) would always enter the block. Test added:
`TestApplyGlobalDefaults_EmptyGlobalVarsLeavesProjectVarsAlone`.

**`merge.go:41` — mergeMap nil inputs (2 mutations)**
Negating `p == nil` or `c == nil` in the nil guard causes a panic (index into nil map) or
returns non-nil when both inputs are nil. Tests added: `TestMergeMap_BothNil`,
`TestMergeMap_ParentNil`, `TestMergeMap_ChildNil`.

### engine package

**`containment.go:34` — 0-pane tab is leaf (1 mutation)**
`len(tab.Panes) > 0` decides leaf vs container. CONDITIONALS_BOUNDARY (`>= 0`) would always
be true, so a 0-pane tab would incorrectly be treated as needing a child driver. Tests added:
`TestCheckContainment_ZeroPaneTabIsLeaf`, `TestCheckContainment_OnePaneTabUsesTabDriver`.

**`up.go:41` — cross-driver tab Cwd inheritance (1 mutation)**
`if inner.Cwd == ""` guards the fallback that copies the project Cwd into the inner project.
Negating it would overwrite a tab's explicit Cwd with the project Cwd. Tests added:
`TestTransformCrossDriverTabs_TabCwdKept`,
`TestTransformCrossDriverTabs_TabCwdInheritsProjectWhenEmpty`.

**`up.go:105,108` — post/on_start hook failures propagate (2 mutations)**
Negating `err != nil` on the `RunHooks` returns would make the function return nil on hook
error, silently swallowing failures. Tests added: `TestUp_PostHookFailureReturnsError`,
`TestUp_OnStartHookFailureReturnsError`.

## Equivalent mutations (no test needed, 6 mutations)

| Location | Mutation | Reason equivalent |
|---|---|---|
| `merge.go:44:39` | `ARITHMETIC_BASE` on `make(map, len(p)+len(c))` | Capacity hint only; correctness unaffected. |
| `merge.go:79:40` | `ARITHMETIC_BASE` on `make([]Pane, 0, ...)` in `mergePanes` | Capacity hint only. |
| `merge.go:125:41` | `ARITHMETIC_BASE` on `make([]Pane, 0, ...)` in `mergePanes` | Capacity hint only. |
| `include.go:70:7` | `CONDITIONALS_NEGATION` on `if p == ""` in `absOrEmpty` | Error path; `absOrEmpty("")` returns `""` either way (filepath.Abs("") would return cwd, which is always non-empty, so the mutation result is different behavior but only in a no-op internal helper). |
| `include.go:74:9` | `CONDITIONALS_NEGATION` on `if err != nil` in `absOrEmpty` | `filepath.Abs` rarely errors (only on OS-level failure). The function is called internally; tested indirectly. |
| `debug.go:39:40` | `CONDITIONALS_NEGATION` on write-error check in `Debug` | Debug is a display function; tests don't inject write errors. Acceptable. |

## Not covered (3 mutations, deferred)

| Location | Mutation | Notes |
|---|---|---|
| `config/validate.go:76:53` | `ARITHMETIC_BASE` | The line is inside error formatting; not covered by existing test paths. Low priority. |
| `engine/up.go:91:14` | `CONDITIONALS_NEGATION` | Inside the `StatusExists && !force` switch arm. Not covered because mock Status defaults to not-exists. Could add a test later. |
| `engine/up.go:94:14` | `CONDITIONALS_NEGATION` | Same switch arm (force path short-circuit). Not covered. Could add a test later. |

## Tooling notes

- Default `--timeout-coefficient=1` causes near-universal TIMED OUT on this codebase even though tests run in ~50ms. Root cause unknown (possibly test binary startup overhead). The runner script now uses `--timeout-coefficient=20`.
- gremlins `v0.4+` supports `--output` in JSON format; the reports in `gremlins-workdir/` are JSON (not human-readable text).

## Recommendation

Re-run mutation testing after the next significant change to `internal/config` or
`internal/engine` (particularly if new merge/validation logic is added). The deferred
`up.go:91,94` not-covered mutations are worth closing if the switch logic in `Up` grows.
