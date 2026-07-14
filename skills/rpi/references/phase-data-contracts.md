# Phase Data Contracts

How each consolidated phase passes data to the next. Artifacts are filesystem-based; no in-memory coupling between phases.

| Transition | Output | Extraction | Input to Next |
|------------|--------|------------|---------------|
| → Discovery | Goal string + repo execution profile contract | Goal from the `/rpi` invocation; repo policy from `docs/contracts/repo-execution-profile.md`, `repo-execution-profile.schema.json`, and `repo-execution-profile.json` when present | `repo_profile` state is loaded before research/planning begins, including validation lane mutation metadata |
| Discovery → Crank | Epic execution context or file-backed objective + discovery summary + `execution_packet` | `phased-state.json` + `.agents/rpi/phase-1-summary.md` + `.agents/rpi/execution-packet.json` (latest alias) or `.agents/rpi/runs/<run-id>/execution-packet.json` (run archive) | `/crank <epic-id>` when `epic_id` exists; otherwise `/crank .agents/rpi/execution-packet.json` with repo policy, contract surfaces, validation bundle, and `validation_lanes` already normalized |
| Crank → Validate | Completed/partial crank status + implementation summary + `execution_packet` | `ao beads exec children <epic-id>` or file-backed implementation state + `.agents/rpi/phase-2-summary.md` + `.agents/rpi/execution-packet.json` (latest alias) or `.agents/rpi/runs/<run-id>/execution-packet.json` (run archive) | `/validate <epic-id>` when `epic_id` exists; otherwise standalone `/validate` with the same repo execution profile fields, validation lanes, and done criteria |
| Validate → Learn | Immutable validation verdict + evidence references + `execution_packet` | `.agents/rpi/phase-3-summary.md` plus the schema-valid verdict artifact | `/learn` with the verdict reference; Learn may emit observations but cannot mutate proof or delivery state |
| Learn → Orchestrator | Learn receipt + bounded observations + `remaining_work` + `plan_impact` | `.agents/rpi/phase-4-summary.md` + latest Learn receipt | `material_change` lets the orchestrator change the remaining plan through Discovery and Premortem; `no_change` requires an explicit retry/continue/stop/escalate decision; `terminal` closes the tick |

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
- every phase boundary artifact records `skills_loaded`
- `.agents/rpi/execution-packet.json` carries exactly four ordered receipts: discovery, crank, validate, learn
- `phase_receipts[].status` must match the delegated skill's completion marker or verdict (`DONE`, `PARTIAL`, `BLOCKED`, `FAIL`, or `PASS/WARN/FAIL` as emitted)
- a `prospective` Discovery handoff records discovery `DONE` with its real artifact, crank `pending`, and validate/learn `not_checked`; pending/not-checked receipts omit `artifact`
- prospective `skills_loaded` names only RPI and Discovery; unrun receipt placeholders do not claim future skill loads
- a `prospective` packet cannot claim successful downstream receipts
- before Report, `packet_state` is `terminal` and the ordered receipts must be successful: discovery `DONE`, crank `DONE`, validate `PASS`, and learn `DONE`, each with an existing nonempty artifact; retry history belongs in phase evidence rather than extra receipts
- receipts are an audit index, not proof by themselves; transcript or runtime invocation trace remains the stronger evidence when available

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
      "artifact": ".agents/rpi/phase-1-summary.md"
    },
    {
      "phase": "crank",
      "skill": "crank",
      "status": "DONE",
      "artifact": ".agents/rpi/phase-2-summary.md"
    },
    {
      "phase": "validate",
      "skill": "validate",
      "status": "PASS",
      "artifact": ".agents/rpi/phase-3-summary.md"
    },
    {
      "phase": "learn",
      "skill": "learn",
      "status": "DONE",
      "artifact": ".agents/rpi/phase-4-summary.md"
    }
  ]
}
```

Markdown phase summaries include a `## Skill Receipts` section with one bullet per loaded skill, the phase it served, and the artifact/verdict it produced.

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
