# Phase Data Contracts

How each consolidated phase passes data to the next. Artifacts are filesystem-based; no in-memory coupling between phases.

| Transition | Output | Extraction | Input to Next |
|------------|--------|------------|---------------|
| → Discovery | Goal string + repo execution profile contract | Goal from the `/rpi` invocation; repo policy from `docs/contracts/repo-execution-profile.md`, `repo-execution-profile.schema.json`, and `repo-execution-profile.json` when present | `repo_profile` state is loaded before research/planning begins, including validation lane mutation metadata |
| Discovery → Crank tranche | Admitted tranche plan + `execution_packet` + bound Premortem verdict | `.agents/rpi/execution-packet.json` (latest alias) or `.agents/rpi/runs/<run-id>/execution-packet.json` (run archive) | `/crank <epic-id>` or `/crank <packet>` for one of at most three admitted waves |
| Crank wave → Orchestrator | Canonical targeted wave evidence + remaining-plan facts | `.agents/swarm/results/<wave>.json` referenced from the execution packet | Reuse the bound Premortem and admit the next unchanged low-risk wave, or REPLAN; do not invoke Validate/Learn per wave |
| Frozen tranche → Validate | Exact candidate identity + claim map + verified deterministic receipts | execution packet plus canonical Crank evidence | one fresh `/validate` after the tranche freezes |
| Validate → Learn | Immutable schema-valid `result.json` | verdict path and digest | `/learn` once; Learn may emit observations but cannot mutate proof or delivery state |
| Learn → Orchestrator | Canonical `learn-receipt.json` with bounded observations, `remaining_work`, and `plan_impact` | receipt path and digest | choose the next tranche, stop, or deliver; optional phase summaries are link-only projections |

Execution packet v1 should remain additive. Recommended fields:
- `schema_version`
- `packet_state` (`prospective` before all phases complete; `terminal` only after Learn)
- `run_id`
- `objective`
- `epic_id` (optional when the tracker cannot mint an epic)
- `plan_path`
- `contract_surfaces`
- `validation_commands`
- `validation_lanes` (repo profile lane metadata: `read_only`, `writes_artifacts`, `isolated_agents_home`, `release_only`, `mutation_escape_hatch`)
- `tracker_mode`
- `tracker_health`
- `done_criteria`
- `premortem_verdict`
- `test_levels`
- `ranked_packet_path`
- `skills_loaded` (canonical skill slugs without sigils; at minimum `rpi` and the delegated phase skill that produced the artifact)
- `phase_receipts` (phase, skill, status/verdict, artifact path, optional next action)

Execution packet retention rule:
- `.agents/rpi/execution-packet.json` is the mutable latest alias for the current objective
- `.agents/rpi/runs/<run-id>/execution-packet.json` is the durable per-run packet archive when `run_id` exists

Phase receipt rule:
- `.agents/rpi/execution-packet.json` carries one ordered index with four typed responsibility receipts: discovery, crank, validate, learn
- each entry points at its canonical artifact; child artifacts do not repeat `skills_loaded` or the full receipt list
- `phase_receipts[].status` must match the delegated skill's completion marker or verdict (`DONE`, `PARTIAL`, `BLOCKED`, `FAIL`, or `PASS/WARN/FAIL` as emitted)
- a `prospective` Discovery handoff records discovery `DONE` with its real artifact, crank `pending`, and validate/learn `not_checked`; pending/not-checked receipts omit `artifact`
- prospective `skills_loaded` names only RPI and Discovery; unrun receipt placeholders do not claim future skill loads
- a `prospective` packet cannot claim successful downstream receipts
- before Report, `packet_state` is `terminal` and the ordered receipts must be successful: discovery `DONE`, crank `DONE`, validate `PASS`, and learn `DONE`, each with an existing nonempty canonical artifact; retry history belongs in phase evidence rather than extra receipts
- receipts are an audit index, not proof by themselves; transcript or runtime invocation trace remains the stronger evidence when available
- `.agents/rpi/phase-{1,2,3,4}-summary*.md`, when required by an older consumer, contains only status, canonical artifact reference/digest, and next action; it never restates findings or analysis

Run disposition rule:
- a next-move decision is a standalone immutable document conforming to
  `skills/rpi/schemas/run-disposition.schema.json`;
- it binds the stable run ID, objective identity/digest, one of
  `NOTE|REPAIR|REPLAN|HOLD|ANDON`, reason, and evidence digests;
- phase receipts may cite the disposition but do not copy it or grow counters,
  reservations, cost state, or helper state; and
- the orchestrator owns the decision. Phase skills only return evidence.

Receipt shape (JSON artifacts use canonical skill slugs without sigils):

```json
{
  "skills_loaded": [
    {"name": "rpi", "reason": "orchestrator"},
    {"name": "discovery", "reason": "phase-1"},
    {"name": "crank", "reason": "phase-2"},
    {"name": "validate", "reason": "phase-3"},
    {"name": "learn", "reason": "phase-4"}
  ],
  "phase_receipts": [
    {
      "phase": "discovery",
      "skill": "discovery",
      "status": "DONE",
      "artifact": ".agents/rpi/execution-packet.json"
    },
    {
      "phase": "crank",
      "skill": "crank",
      "status": "DONE",
      "artifact": ".agents/swarm/results/tranche.json"
    },
    {
      "phase": "validate",
      "skill": "validate",
      "status": "PASS",
      "artifact": ".agents/council/result.json"
    },
    {
      "phase": "learn",
      "skill": "learn",
      "status": "DONE",
      "artifact": ".agents/rpi/learn-receipt.json"
    }
  ]
}
```

Compatibility Markdown summaries are generated views of these entries; they do
not carry another `## Skill Receipts` copy or a second analysis.

Validation lane selection rule:
- implementation and fast closeout phases prefer lanes where `read_only=true`, `writes_artifacts=false`, `release_only=false`, `cost_class` is `cheap` or `standard`, and `auto_select` is `default` or matches the changed surface
- lanes with `cost_class=expensive`, `auto_select=explicit`, or `auto_select=release-only` require an explicit operator request, a named plan acceptance criterion, or a release-readiness objective
- every selected lane should honor `timeout_seconds`; when a lane times out, record `[TIME-BOXED]` and continue with narrower evidence unless it was the only code-surface proof
- lanes with `isolated_agents_home=true` require isolated agent state before execution
- lanes with `writes_artifacts=true` or `release_only=true` are release/audit lanes; run them only when the packet objective or operator explicitly asks for release readiness
- lanes with a non-null `mutation_escape_hatch` require the escape hatch name in the validation report or handoff
- unclassified commands containing `go test -race`, `-shuffle`, `-count=N` where `N > 1`, eval runners, retrieval bench, headless runtime smoke, or release gates are explicit-only

Queue lifecycle rule:
- postmortem writes new entries as available: entry aggregate `consumed=false`, `claim_status="available"`
- consumers treat item lifecycle as authoritative inside `items[]`; omitted item `claim_status` means available
- `/evolve` and `/rpi loop` claim an item before starting a cycle: item `claim_status="in_progress"`
- successful `/rpi` + regression gate finalizes that item claim: item `consumed=true`, `claim_status="consumed"`, `consumed_by`, `consumed_at`
- failed or regressed cycles release the claim back to available state and may stamp item `failed_at` for retry ordering
- consumers may rewrite existing queue lines to claim, release, fail, or consume items after initial write
- the entry aggregate flips to `consumed=true` only after every child item is consumed

Canonical schema contract: [`docs/contracts/next-work.schema.md`](../../../docs/contracts/next-work.schema.md) (v1.4)

## Rollback discipline

When changing phase-boundary logic, ship phase-3 (closeout) updates FIRST and remove phase-2 (handoff) logic SECOND. This keeps a working closeout path in place during the transition window — if phase-3 ships broken, you can revert before phase-2's removal lands.

Premortem F3 of `soc-bcrn` (`.agents/council/2026-05-07-pre-mortem-rpi-lifecycle-sharpening.md`) called this out as the primary rollback risk for the consolidated daemon epic (E3).

Worked example: `cli/cmd/ao/rpi_cleanup.go:preserveWorktreeCommits` (phase-3 commit-preservation logic) was added BEFORE the phase-2 cleanup-removal as part of E3.S3. The order matters: orphaned worktree commits that previously fell into git fsck dangling now land on `codex/preserve-<runID>` branches before the worktree is force-removed. If preservation had been wired in the opposite order, a regression window would have lost commits during the transition.
