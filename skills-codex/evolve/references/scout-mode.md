# Scout Mode — First-Class Cycle Result

Scout mode is a result type alongside `improved`, `regressed`, `harvested`, and `idle`. A scout cycle reads a candidate work item, validates its scope and shape, and either annotates the queue entry with a deeper plan or splits it into smaller beads — without executing the underlying work.

## When to scout

Use scout-mode whenever a Step 3 selection meets any of these criteria:

- The work touches **> 5 files** and is **not** a mechanical batch (a single script-driven rewrite across N similar files).
- The work introduces a **new shape**: schema field, frontmatter carrier, JSON top-level key, validator rule, contract surface, or struct field that downstream consumers will read.
- The work is **operator-level epic work**: multiple cooperating sub-systems must change together (e.g. emitter + consumer + validator + tests + docs in one cycle).
- The **current cycle is > 5 productive cycles into the session** and the work would extend the implementation arc rather than close it.

The scope-filter step (Step 3.0 in `SKILL.md`) consults these heuristics before any work is claimed.

## What a scout cycle does (soc-5qit: split-or-defer, never bail)

A scout cycle is **work**, not a stop. It MUST produce exactly one of:

### Path A: Split (preferred when the queue is light and the work is decomposable)

1. Read the target file(s) named in the work item (no edits).
2. Map the **current shape** at the relevant boundary (what fields exist, what callers read it, what validators enforce).
3. Run `ao beads exec create` to decompose the candidate into 2-N child beads, each ≤5 files and single-shape:
   ```bash
   ao beads exec create "Slice 1 of <parent-title>: <smaller-scope>" \
     --description="Carved from <parent-id> by scout-mode. Scope: <files/contract>" \
     --deps discovered-from:<parent-id> -t task -p <inherit> --json
   ```
4. Update the parent bead with `ao beads exec update <parent-id> --notes "scout-split into: <child-ids>"`.
5. **Re-enter Step 3** so the smallest new child OR another ready bead gets claimed THIS cycle.

### Path B: Defer (preferred when the queue has other ready beads)

1. Read the target file(s) briefly to confirm the scope assessment.
2. Append a `disposition: defer:<reason>` block to the work item.
3. **Re-enter Step 3** so the next-priority ready bead gets claimed THIS cycle. The big candidate stays available for a future session with lighter context.

### Path C: Park (operator-level epic, no obvious split)

1. `bd update <id> --status blocked --notes "scope-too-big: <why>; needs operator triage"`.
2. **Re-enter Step 3** for the next ready bead.

A scout cycle does NOT:

- Run `/rpi` against the original too-big candidate.
- Edit any source file outside `.agents/rpi/next-work.jsonl` and bd metadata.
- Commit code (bd updates land in Dolt automatically).
- **Exit the loop** — if `bd ready` returns ≥1 unblocked bead, the cycle MUST claim and work one of them after the scout decision. (soc-5qit invariant.)

## Logging a scout cycle

Append to `cycle-history.jsonl` with:

```json
{"cycle": N, "result": "scout", "selected_source": "<source>", "work_ref": "<id>",
 "net_change": 0, "commit": null,
 "milestone": "Scouted <work>; recommendation: <split|park|smaller-slice>"}
```

The `result: scout` value is canonical alongside `improved | regressed | harvested | idle | unchanged`.

## Daily learning capture

Scout cycles still get a micro-capture line. Use the form:

```
- cycle N [scout] <work-ref>: <what was learned about the shape>  INSIGHT: <tag>
```

## Promotion path

When a scouted item later becomes single-cycle-doable (because earlier prerequisites landed), drop the `disposition` block and let normal Step 3.1 selection pick it up.

## Why scout is not "idle"

`idle` means "no actionable work in any layer". Scout means "actionable work found but the shape is wrong for this cycle's budget". These are structurally different stop reasons. Conflating them masks the real failure mode: the loop has work but can't safely run it.
