# Runbook: Outcomes grading (holdout-safe Outcomes projection)

> Operator guide for the AgentOps × Claude Managed Agents **Outcomes** lane —
> grading an agent run against a *rubric* (the Anthropic Managed Agents Outcomes
> analog) instead of an exact-match answer, as a **holdout-safe projection** of
> the LOCKED eval substrate (`~/.agents/evals/SCHEMA.md`).
>
> **Scope:** this runbook documents the holdout-safety **gates** that ship on
> `main` (compile → ingest → gate #2 → gate #3). The end-to-end Knowledge-Flywheel
> close (emitting a verdict-compiler run-manifest) is a separate, still-open slice
> — see "Not yet wired" below.

## The load-bearing rule: Managed Agents are NOT ZDR

Anthropic Managed Agents are **not** Zero-Data-Retention. A holdout answer
(`target` / `ground_truth` / a `samples-holdout.jsonl` value) that crosses the
cloud boundary is **permanently** exposed and the holdout split is statistically
spent. Everything below exists to make that impossible by construction, by
re-scan, and by schema. **When in doubt, the holdout never ships.**

## When to use cloud Outcomes vs local

| Use **cloud Outcomes** (Managed Agents) | Use **local / Qwen3.6** (Codex, offline) |
|---|---|
| Async / long-running grading jobs | Cost-sensitive or high-volume grading |
| You want Anthropic's hosted grader | Offline / air-gapped, or no cloud budget |
| The rubric is **dev-split** or already holdout-stripped | Holdout-split grading (keep it on-box) |

Either way the score ingests through the **same** `ao eval outcomes ingest` path
and lands in the one council verdict format — there is no second bar.

## The flow (commands on `main`)

```
ao eval outcomes compile  <input.json>                  # locked Task+criteria → holdout-safe Rubric payload
   │  (cross the boundary; grade with the runtime-appropriate grader)
   ▼
ao eval outcomes ingest   <score.json> [gates...]       # score → one council verdict (+ gate #2/#3)
```

### 1. Compile — `ao eval outcomes compile <input.json>`

Projects a locked Task + its grading criteria into a **holdout-safe Rubric**
(`schemas/outcomes-rubric.v1.schema.json`): only `source_task_id`,
`judge_content_hash`, allowlisted `criteria[]` (id/description/weight), and
optional `instructions`. It **refuses** to emit a rubric that would carry any
holdout value (re-scan guard; `ProjectRubric` cannot copy ground truth by
construction). The schema sets `additionalProperties:false` at **every** nesting
level, so a malformed payload carrying a `target`/`ground_truth`/`expected_output`
fails validation, not just a code check.

### 2. Ingest — `ao eval outcomes ingest <score.json>`

Maps an Outcomes score onto the one council verdict record (PASS/WARN/FAIL from
the aggregate vs the rubric threshold). Two opt-in holdout-safety gates wrap it:

#### Gate #2 — rubric-drift parity (`--expect-judge-hash <hash>`)

A score carries the `judge_content_hash` of the rubric it was graded against
(SCHEMA rc2 drift key). Pass the **active** rubric's hash and ingest **refuses**
on a mismatch — a drifted rubric self-invalidates exactly like a stale local
judge:

```
ao eval outcomes ingest score.json --expect-judge-hash sha256:<active-rubric-hash>
```

If you **omit** `--expect-judge-hash` but the score carries a hash, ingest still
succeeds but emits a **`WARN ... gate #2 was NOT checked`** to stderr — the
unenforced-parity gap is loud, not silent. Prefer always passing the flag in
automated flows.

#### Gate #3 — holdout-burn quota (`--burn-ledger <path>`)

When grading against the **holdout** split, set a burn-ledger path. A holdout
score registers exactly one burn in the persisted ledger and is **REFUSED** once
the `(suite, gt_version)` quota is exhausted — so a cloud/Codex Outcomes run
cannot silently re-observe a spent holdout split. Persisted across invocations
(atomic write):

```
ao eval outcomes ingest score.json --burn-ledger ~/.agents/evals/burn/<suite>.json
```

The ledger is a JSON `HoldoutBurnLedger` (`{budget, records[]}`); seed `budget`
once. A non-positive budget = no ceiling (no-op). **Dev-split scores never burn**
— the dev split is reusable. An empty `--burn-ledger` leaves enforcement off
(dev/legacy flows unaffected).

## Holdout-safety guard summary

| Layer | Guard | Where |
|---|---|---|
| Schema | `additionalProperties:false` at every level forbids leak fields | `schemas/outcomes-rubric.v1.schema.json` |
| By construction | `ProjectRubric` never copies ground truth | `cli/internal/evalsubstrate/rubric.go` |
| Re-scan | `Rubric.ContainsAny` refuses any holdout value in the payload | compile path |
| Gate #2 | rubric-drift parity refuse + unchecked-parity warn | `--expect-judge-hash` |
| Gate #3 | holdout-burn quota refuse + persistence | `--burn-ledger` |

## Not yet wired (follow-on)

- **Knowledge-Flywheel close (run-manifest):** emitting a verdict-compiler
  run-manifest so an Outcomes verdict mutates `.agents/learnings/` frontmatter
  end-to-end. Tracked on **ag-fy6e** (needs the verdict-compiler contract + a
  holdout-safety decision on which manifest fields an out-of-process grade may
  carry — the run-manifest's `ground_truth_ref`/`ground_truth_hash` must never
  carry holdout ground truth).
- **Shared cross-host burn ledger** (cross-host quota): **ag-vwiv**. Note: Dolt is retired (2026-06-11) as the cross-host backend, so this follow-on needs a Dolt-free mechanism; the local per-suite `--burn-ledger` file remains the only enforced quota today.
- **`--score/--suite/--bead` flag interface + validation/ratchet skill wiring:**
  remaining slices of **ag-62g68** / **ag-ko5rj**.
- **Codex/NTM Qwen3.6 grader** producing an identical outcomes-score: **ag-2hi41**.

## See also

- [Outcomes Rubric Projection Contract](../contracts/outcomes-rubric-projection.md)
- [Eval Verdict Pipeline Contract](../contracts/eval-verdict-pipeline.md)
- `~/.agents/evals/SCHEMA.md` — the LOCKED substrate this lane projects from (never relitigated)
