# Cloud-Frontier Factory Pilot Runbook

This runbook is the milestone-1 pilot for comparing one cloud/frontier coding
worker against two cloud/frontier coding workers. It is bounded, manual-merge
only, and does not use GasCity / Mt. Olympus production routing.

## Build The Plan

```bash
ao factory pilot --goal "your bounded objective" --json
```

The command prints:

- a baseline phase with one `frontier-codex` worker;
- a treatment phase with two `frontier-codex` workers;
- explicit max concurrency of `1` for baseline and `2` for treatment;
- daemon-style slot and worktree allocations;
- required validation commands;
- lifecycle event templates;
- `factory.yield_observation` templates;
- disabled reference lanes;
- manual merge and retention instructions.

## Dispatch Rules

- Use only cloud/frontier `openai` + `codex` coding workers.
- Keep GasCity / Mt. Olympus lanes disabled for production task classes.
- Allocate one isolated worktree per slot.
- Preserve failed worktrees and artifacts by default.
- Do not merge automatically.

## Required Events

Each worker slot needs this event sequence:

1. `factory.job_submitted`
2. `factory.routing_decided`
3. `factory.slot_allocated`
4. `factory.worktree_allocated`
5. `factory.validation_started`
6. `factory.validation_completed`
7. `factory.merge_decision`
8. `factory.yield_observation`

If validation fails, record `factory.job_terminal` with
`retained_worktree: true`, artifact refs, log refs, transcript refs, and diff
refs before any recovery attempt.

## Manual Merge Review

Merge review can start only when every command listed in the pilot plan's
`validation_commands` has a passed validation event. Then follow
[`factory-manual-merge.md`](factory-manual-merge.md).

Treat missing validation, missing diff, missing transcript, or missing yield
observation as a failed gate. The correct outcome is `rejected`, `abandoned`,
or a new repair job, not silent cleanup.

## Yield Comparison

Compare baseline and treatment with the factory yield ledger:

- accepted patches per hour;
- model/API cost;
- wall-clock, review, and recovery minutes;
- conflict count;
- defect count;
- operator interventions;
- advisory sidecar consumption and decision-use links.

The treatment wins only if it improves accepted validated work per hour after
review and recovery time are included and does not increase defects or hidden
operator intervention.

