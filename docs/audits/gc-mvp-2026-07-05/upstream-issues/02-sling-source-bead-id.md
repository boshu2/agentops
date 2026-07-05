# `gc sling <target> <bead> --on <v2-formula>` does not stamp `gc.source_bead_id`, so workflow-finalize cannot close the source bead

## Environment

- gascity edge (post-1.3.3), source snapshot `8b17c64`
- macOS (darwin/arm64), `gc` built from source
- `GC_BEADS=file` (observed there; the code path is backend-independent)

## Summary

Attaching a **formulas-v2 (graph) formula** to an existing bead with
`gc sling <target> <bead-id> --on <formula>` launches the workflow, but the
workflow root never receives `gc.source_bead_id`. As a result, when the
workflow finalizes with outcome pass, `closeSourceBeadChain` finds no source
metadata and stops as a no-op — the source bead the formula was attached to
stays open forever and must be closed by hand.

The legacy (non-graph) `--on` path stamps the source correctly; only the v2
graph branch drops it.

## Minimal repro

1. Create a city with a v2 formula (any `version = 2` graph formula) and one
   agent that can run it.
2. `bd create` (or `gc bd create`) a work bead; note its id, e.g. `gc-101`.
3. `gc sling <agent> gc-101 --on <v2-formula>` — the workflow instantiates and
   starts.
4. Inspect the workflow root bead's metadata.
5. Let the workflow run to completion (all steps pass, `workflow-finalize`
   fires).

**Observed:** the root has no `gc.source_bead_id` (step 4); after finalize the
root closes but `gc-101` remains open (step 5). The finalize trace logs
`close-source-chain ... stop reason=no_source`.

**Expected:** the root carries `gc.source_bead_id=gc-101` and finalize closes
`gc-101` along with the root, as the non-graph `--on` path does.

## Code pointers (control flow at `8b17c64`)

- `internal/sling/sling_core.go:295` — `slingOnFormula`. For a v2 formula,
  `isGraph` is true and the function returns inside the
  `withGraphV2SourceWorkflowLock` closure (lines 313-341). That closure calls:
  - `InstantiateSlingFormula(..., "", opts.ScopeKind, ...)` at line 324-328 —
    the 4th positional arg is `sourceBeadID`, passed as **`""`**;
  - `doStartGraphWorkflow(mResult.RootID, "", a, method, deps)` at line 332 —
    `sourceBeadID` again **`""`**, even though `beadID` is in scope (it is
    used for the lock and conflict checks just above).
- `internal/sling/sling_core.go:654` — `doStartGraphWorkflow` only stamps
  `gc.source_bead_id` / `gc.source_store_ref` and repoints the source bead's
  `workflow_id` inside `if sourceBeadID != ""`, so the stamping code exists
  but never fires on this path.
- Contrast: the legacy branch at `sling_core.go:366` passes `beadID` into
  `doStartGraphWorkflow`, and `internal/graphroute/graphroute.go:600-605`
  likewise stamps the root step only when `sourceBeadID != ""`.
- `internal/sling/sling_core.go:444` — `slingDefaultFormula` has the identical
  empty-`sourceBeadID` call in its `isGraph` branch (default `--on` formula),
  so the default-formula attachment path is affected too.
- Consumer that breaks: `internal/dispatch/runtime.go:753` +
  `closeSourceBeadChain` (`runtime.go:787`, walk at `runtime.go:810`) — the
  walk reads `gc.source_bead_id` from the root and stops with
  `reason=no_source` when it is absent.

## Suggested fix

Pass `beadID` as `sourceBeadID` in the graph-v2 branches of `slingOnFormula`
(sling_core.go:328 and :332) and `slingDefaultFormula` (:440 and :444), as the
legacy path already does at :360/:366.
