# Gate 4 Loop and Spawn Next Work

## Post-Validation Loop (Optional) -- Post-mortem to Spawn Another /rpi

**Default behavior:** /rpi ends after Validation (Phase 3).

**Enable loop:** pass `--loop` (and optionally `--max-cycles=<n>`).

**Gate 4 goal:** make the "ITERATE vs TEMPER" decision explicit, and if iteration is required, run another full /rpi cycle with tighter context.

**Loop decision input:** the most recent post-mortem council verdict.

1. Find the most recent post-mortem report:
   ```bash
   REPORT=$(ls -t .agents/council/*post-mortem*.md 2>/dev/null | head -1)
   ```
2. Read `REPORT` and extract the verdict line (`## Council Verdict: PASS / WARN / FAIL`).
3. Apply gate logic (only when `--loop` is set). If verdict is PASS or WARN, stop (TEMPER path). If verdict is FAIL, iterate (spawn another /rpi cycle), up to `--max-cycles`.
4. Iterate behavior (spawn). Read the post-mortem report and extract 3 concrete fixes, then re-invoke /rpi from Phase 1 with a tightened goal that includes the fixes:
   ```
   /rpi "<original goal> (Iteration <n>): Fix <item1>; <item2>; <item3>"                 # default strict-quality path (test-first on)
   /rpi "<original goal> (Iteration <n>): Fix <item1>; <item2>; <item3>" --no-test-first # explicit opt-out path
   ```
   If still FAIL after `--max-cycles` total cycles, stop and require manual intervention (file follow-up bd issues).

## Spawn Next Work (Optional) -- Post-mortem to Queue Next RPI

**Enable:** pass `--spawn-next` flag.

**Complementary to Gate 4:** Gate 4 (`--loop`) handles FAIL->iterate (same goal, tighter). `--spawn-next` handles PASS/WARN->new-goal (different work harvested from post-mortem).

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

     Post-mortem harvested N follow-up items from <source_epic>:
     1. <title> (severity: <severity>, type: <type>)
     ...

     To start the next RPI cycle:
       /rpi "<highest-severity item title>"
     ```
   - Do NOT auto-invoke `/rpi` -- the user decides when to start the next cycle
3. If no unconsumed entries: report "No follow-up work harvested. Flywheel stable."

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
