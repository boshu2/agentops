# Scout Mode — First-Class Cycle Result

Scout mode is a result type alongside `improved`, `regressed`, `harvested`, and `idle`. A scout cycle reads a candidate work item, validates its scope and shape, and either annotates the queue entry with a deeper plan or splits it into smaller beads — without executing the underlying work.

## When to scout

Use scout-mode whenever a Step 3 selection meets any of these criteria:

- The work touches **> 5 files** and is **not** a mechanical batch (a single script-driven rewrite across N similar files).
- The work introduces a **new shape**: schema field, frontmatter carrier, JSON top-level key, validator rule, contract surface, or struct field that downstream consumers will read.
- The work is **operator-level epic work**: multiple cooperating sub-systems must change together (e.g. emitter + consumer + validator + tests + docs in one cycle).
- The **current cycle is > 5 productive cycles into the session** and the work would extend the implementation arc rather than close it.

The scope-filter step (Step 3.0 in `SKILL.md`) consults these heuristics before any work is claimed.

## What a scout cycle does

A productive scout cycle MUST:

1. Read the target file(s) named in the work item (no edits).
2. Map the **current shape** at the relevant boundary (what fields exist, what callers read it, what validators enforce).
3. Identify the **smallest landable slice** — usually a producer-side change OR a consumer-side change, not both.
4. Append a `disposition:` block to the work item's `description` in `.agents/rpi/next-work.jsonl` summarizing the finding, with timestamp.
5. If the scope is genuinely epic-level (operator decision needed), explicitly recommend `bd create` for each sub-slice and either `park` or annotate the bead for operator-driven `/rpi`.

A scout cycle does NOT:

- Run `/rpi` against the target.
- Edit any source file.
- Commit (the appended `disposition` lives in `.agents/rpi/next-work.jsonl` which is the standard append-only ledger; commit it as part of the next productive cycle's harvest if appropriate).

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
