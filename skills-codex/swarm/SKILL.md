---
name: swarm
description: 'Execute authorized parallel Codex lanes. Triggers: "$swarm", "parallel wave", "dispatch workers".'
---
# $swarm — Conflict-Safe Wave Execution

Execute an explicitly authorized parallel wave. Default to sequential work
unless at least two independent lanes and disjoint ownership are proven.

## Critical Constraints

- **Why: avoid orchestration tax.** Runtime capability does not authorize fan-out.
- **Why: prevent collisions.** Assign issue id, files, output, validation, base,
  owner, worktree, and discard path before spawn.
- **Why: derived surfaces collide.** Manifests, registries, schemas, migrations,
  CLI surfaces, fixtures, and generated files count as writes.
- **Why: keep evidence durable.** Workers write results to disk with RED,
  commit, test tail, changed files, and conflicts.
- **Why: bound failure.** Cap a wave at 4-6 workers and retry each task at most twice.
- **Why: preserve operator choice.** NTM, Agent Mail, managed agents, and GC are
  used only when the operator explicitly selected that substrate.

## Local Mode and Workflow

1. Confirm explicit parallel authorization and at least two valuable lanes.
2. Build task packets with `metadata.issue_type`, exact file manifests,
   dependencies, validation, result path, base SHA, and cleanup plan.
3. Run pre-spawn friction gates and reject all ownership overlap, including
   generated companions. Display the ownership matrix.
4. Select the authorized backend. If spawning is unavailable or invalid, run
   sequentially with the same contracts.
5. Assign explicit ownership per worker before spawning: issue id, file set, and expected output.
6. Give each worker one isolated task/worktree; out-of-scope discoveries become
   `.agents/swarm/scope-escapes.jsonl`, not edits.
7. Use file-backed result handoff under `.agents/swarm/` for consolidation and deterministic merge order.
8. Validate RED→green evidence, commit persistence, changed paths, tests,
   conflicts, project gates, and independent PAWL before closure.
9. Close workers and reap worktrees only after feature ancestry on trunk.

## Codex Execution Profile

- Use Codex session agents only after explicit authorization and wave admission.
- Do not give two workers overlapping write ownership in the same wave unless the merge plan is explicit.
- Keep messages short; detailed findings and proof belong in result files.
- Use sequential fallback when file manifests, base state, or backend health are uncertain.

## Guardrails

- Do not spawn because `spawn_agent` happens to be available.
- Do not auto-start NTM, Agent Mail, GC, or another runtime.
- Do not let workers race-claim tasks or negotiate file ownership.
- Do not accept prose summaries without commit, RED, test, and path evidence.
- Do not force-remove an unlanded worktree.

## Worker Result Contract

```json
{"issue_id":"age-x.1","status":"done","files_changed":["path/file"],"commit_sha":"<sha>","red_evidence":"<before failure>","test_tail":"<final output>","conflicts_surfaced":[],"worktree_path":"<absolute>"}
```

## Output Specification

- **Artifact directory:** `.agents/swarm/results/`; scope escapes use JSONL.
- **Filename convention:** one `<issue-id>.json` per lane.
- **Serialization/schema format:** JSON result contract with exact evidence.
- **Validator command:** run `bash skills-codex/swarm/scripts/validate.sh`,
  swarm-evidence validation, project tests, and wave/landing gates.
- **Downstream handoff:** consumed by `$crank`, `$validate`, PAWL, and closeout.

## Quality Rubric

- Parallelism was authorized, useful, and ownership-disjoint.
- Every worker stayed isolated and inside its manifest.
- Results contain reproducible RED, commit, test, path, and conflict evidence.
- Integration and validation order are deterministic.
- Retry and cleanup boundaries are honored.

## Troubleshooting

| Problem | Response |
|---|---|
| overlap found | serialize or merge tasks before spawn |
| backend unavailable | execute sequentially |
| scope escape | reject the edit and record follow-up |
| stalled worker | bounded correction, then close/re-plan |

## References

- [validation-contract.md](references/validation-contract.md) · [pre-spawn-friction-gates.md](references/pre-spawn-friction-gates.md)
- [shared-checkout-discipline.md](references/shared-checkout-discipline.md) · [worktree-isolation.md](references/worktree-isolation.md)
- [worker-pre-task-checks.md](references/worker-pre-task-checks.md) · [worker-pitfalls.md](references/worker-pitfalls.md)
- [conflict-recovery.md](references/conflict-recovery.md) · [scope-escape-template.md](references/scope-escape-template.md) · [cold-start-contexts.md](references/cold-start-contexts.md)
- [backend-codex-subagents.md](references/backend-codex-subagents.md) · [backend-background-tasks.md](references/backend-background-tasks.md) · [backend-inline.md](references/backend-inline.md)
- [local-mode.md](references/local-mode.md) · [ralph-loop-contract.md](references/ralph-loop-contract.md)
