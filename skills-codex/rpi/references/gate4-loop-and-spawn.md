# Orchestrator Loop and Spawn Next Work

## Post-Learn Loop (Optional)

**Default behavior:** `/rpi` ends after Learn and the orchestrator decision.

**Enable loop:** pass `--loop` with the same stable run ID.

**Loop goal:** make retry, continue, stop, escalate, re-plan, and close explicit.

**Loop decision input:** the latest schema-valid Learn receipt, bound to its
immutable Validate verdict.

1. Read the canonical Learn receipt referenced by the execution packet. A
   phase-4 summary, if present, is only a link-only compatibility projection.
2. Verify the receipt's verdict digest and `plan_impact` disposition.
3. Apply the disposition:
   - `material_change` with remaining work: the orchestrator invokes Discovery,
     persists the changed plan, runs Premortem on that plan, then may loop;
   - `no_change` with remaining work: the orchestrator explicitly retries,
     continues, stops, or escalates;
   - `terminal`: close without another Premortem or `/rpi` invocation.
4. Record the next action as one evidence-bound disposition. The orchestrator
   owns the decision; Validate and Learn do not retry, escalate, or dispatch.

## Spawn Next Work (Optional) -- Learn Evidence to Queue Next RPI

**Enable:** pass `--spawn-next` flag.

**Complementary to the loop:** `--loop` continues the same objective after an
explicit orchestrator decision. `--spawn-next` only suggests separately queued
work; it never converts a verdict directly into execution.

1. Read `.agents/rpi/next-work.jsonl` for unconsumed entries (schema contract: [`docs/contracts/next-work.schema.md`](../../../docs/contracts/next-work.schema.md)).
   Filter entries by `target_repo`:
   - **Include** if `target_repo` matches the current repo name, OR `target_repo` is `"*"` (wildcard), OR the field is absent (backward compatibility).
   - **Skip** if `target_repo` names a different repo.
   - Current repo is derived from: `basename` of `git remote get-url origin`, or failing that, `basename "$PWD"`.
2. If unconsumed, repo-matched entries exist:
   - If `--dry-run` is set: report items but do NOT mutate next-work.jsonl (skip consumption). Log: "Dry run -- items not marked consumed."
   - Otherwise: claim the current cycle's item first (item `claim_status: "in_progress"`, `claimed_by: <epic-id>`, `claimed_at: <now>`)
   - Only after the cycle finishes PASS/WARN and clears the regression gate: finalize that item (`consumed: true`, `claim_status: "consumed"`, `consumed_by: <epic-id>`, `consumed_at: <now>`)
   - If the cycle fails, regresses, or is interrupted: release the item claim (`claim_status: "available"`, clear `claimed_by` / `claimed_at`, keep `consumed: false`)
   - Task failures may also stamp item `failed_at`; that is retry-order metadata, not a stop condition
   - Report harvested items to user with suggested next command:
     ```
     ## Next Work Available

     Learn recorded N follow-up candidates from <source_epic>:
     1. <title> (severity: <severity>, type: <type>)
     ...

     To start the next RPI cycle:
       /rpi "<highest-severity item title>"
     ```
   - Do NOT auto-invoke `/rpi` -- the orchestrator or user decides when to start the next cycle
3. If no unconsumed entries: report "No follow-up work recorded."

**Note:** Phase 0 read is read-only. Mutating queue state follows a claim/finalize lifecycle so failed cycles can safely release work back to the queue without blacklisting sibling items in the same harvested batch.

## Repo-Scoped Filtering (target_repo)

Both Phase 0 and `--spawn-next` filter next-work entries by `target_repo`:

| `target_repo` value | Behavior |
|---------------------|----------|
| Matches current repo | Included |
| `"*"` (wildcard) | Included — applies to any repo |
| Absent / null | Included — backward compatible with pre-v1.2 entries |
| Different repo name | Skipped — intended for a different rig |

The current repo name is resolved as: `basename $(git remote get-url origin 2>/dev/null)` with `.git` suffix stripped, falling back to `basename "$PWD"` when no remote is configured.

This prevents cross-repo pollution when `.agents/rpi/next-work.jsonl` is shared or synced across rigs.

## Claim / Release State Machine

| State | Required fields | Meaning |
|-------|-----------------|---------|
| available | item `consumed=false`, item `claim_status="available"` | Ready for `/evolve` or `--spawn-next` to pick |
| in_progress | item `consumed=false`, item `claim_status="in_progress"`, item `claimed_by`, item `claimed_at` | Currently being worked |
| consumed | item `consumed=true`, item `claim_status="consumed"`, item `consumed_by`, item `consumed_at` | Successfully completed and retired from the queue |

Entry-level lifecycle fields are aggregates for dashboards and legacy readers. Never mark an item consumed at pick-time. Claim first, consume on success, release on failure.
