---
name: swarm
description: 'Dispatch parallel agents. Triggers: "swarm", "dispatch parallel agents.", "swarm skill".'
practices:
- microservices
- team-topologies
- mythical-man-month
hexagonal_role: supporting
consumes:
- implement
- validate
produces:
- .agents/swarm/results/*.json
context_rel:
- kind: customer-of
  with: crank
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
metadata:
  tier: orchestration
  dependencies:
  - implement
  - validate
output_contract: .agents/swarm/results/*.json
---
# Swarm Skill

Execute an explicitly authorized parallel wave with fresh workers, disjoint
ownership, file-backed results, and independent validation. Default to one
agent and sequential work when a valid wave cannot be proven.

## Critical Constraints

- **Why: parallelism has overhead.** Swarm only when the operator/workflow asks
  for it and at least two independent lanes materially benefit.
- **Why: prevent collisions.** Every task declares issue id, `metadata.issue_type`,
  exact files, validation, output, owner, base SHA, and discard path before spawn.
- **Why: derived files collide too.** Registry, Codex manifest, schemas,
  migrations, CLI surfaces, and generated projections count as shared writes.
- **Why: avoid shared-checkout corruption.** Give writers isolated worktrees;
  when an explicitly requested shared multi-writer workflow needs reservations,
  reserve through Agent Mail before edits.
- **Why: substrates are operator choices.** Do not start NTM, Agent Mail, Gas
  City, managed agents, or a runtime merely because it exists.
- **Why: self-report is not proof.** Require RED evidence, commit SHA, test tail,
  changed files, and conflicts; independently gate every landed slice.
- **Why: bound failures.** Maximum 4-6 workers per wave and two retries per task;
  scope escapes become follow-up work, never unauthorized edits.

## Local Mode and Routing

| Shape | Route |
|---|---|
| one deliverable or uncertain ownership | sequential current agent |
| read-only independent perspectives, explicitly requested | bounded in-session fan-out |
| ≥2 disjoint working-tree slices | this wave executor |
| persistent pane roles explicitly requested | [`ntm`](../ntm/SKILL.md) + [`agent-native`](../agent-native/SKILL.md) and optional Agent Mail |
| city-shaped durable work explicitly selected | `using-gc`, with membrane close door |

`/crank` owns wave admission; swarm executes admitted waves. Read
[execution-steps.md](references/execution-steps.md) for the full mechanics and
[pre-spawn-friction-gates.md](references/pre-spawn-friction-gates.md) for base,
manifest, dependency, alignment, and wave-cap gates.

## Workflow

1. **Confirm authorization and value.** Require explicit parallel-work intent
   and at least two ready lanes. Otherwise report the sequential route.
2. **Build task packets.** Each task carries id, subject, behavior, exact file
   manifest, dependencies, `metadata.issue_type`, validation/RED command,
   expected result path, base SHA, worktree, and cleanup/discard plan.
3. **Prove wave validity.** Topologically select unblocked tasks; reject any
   write/write or read/write overlap, including generated companions. Serialize
   coupled chains. Display the ownership matrix before spawning.
4. **Choose only the authorized backend.** Codex sub-agents, Claude teams,
   background tasks, inline fallback, NTM+Agent Mail, or GC are adapters—not
   automatic routing authority. Read only the selected backend reference.
5. **Isolate and dispatch.** Create/verify one worktree per writer when needed,
   assign ownership before spawn, and give each worker one bounded task. Workers
   must not claim extra work or edit outside their manifest.
6. **Collect file-backed results.** Workers write
   `.agents/swarm/results/<issue-id>.json`; coordination messages are short
   signals. A scope escape appends `.agents/swarm/scope-escapes.jsonl`.
7. **Validate and integrate deterministically.** Verify persistence, changed
   paths, RED→green evidence, tests, conflicts, and commit ancestry. Merge in
   declared order, run wave-level gates, then route each slice through PAWL.
8. **Retry or stop.** Correct a worker at most twice. On collision, changed
   scope, gate failure, or exhausted retry, stop that lane and re-plan.
9. **Cleanup.** Close ephemeral workers. Reap a worktree only after its feature
   commit is an ancestor of trunk; a closed tracker item alone is insufficient.

## Worker Result Contract

```json
{
  "issue_id": "age-x.1",
  "status": "done",
  "files_changed": ["path/file"],
  "commit_sha": "<sha>",
  "red_evidence": "<failing command/output before implementation>",
  "test_tail": "<verbatim final output>",
  "conflicts_surfaced": [],
  "worktree_path": "<absolute path>"
}
```

Missing `commit_sha`, `red_evidence`, or `test_tail` means unverified. See
[validation-contract.md](references/validation-contract.md) and
[worker-specs.md](references/worker-specs.md).

## Output Specification

- **Artifact directory:** `.agents/swarm/results/`; scope escapes use
  `.agents/swarm/scope-escapes.jsonl`, shared schemas may use output-schema.json.
- **Filename convention:** one `<issue-id>.json` per lane and deterministic
  wave order in the lead summary.
- **Serialization/schema format:** JSON worker result contract plus JSONL scope
  escapes and exact commit/test evidence.
- **Validator command:** run `bash skills/swarm/scripts/validate.sh`,
  `bash scripts/validate-swarm-evidence.sh` when evidence exists, project tests,
  and the wave/landing gates.
- **Downstream handoff:** consumed by `/crank`, `/validate`, PAWL, tracker
  closeout, and `/post-mortem` harvesting.

## Quality Rubric

- Every lane was explicitly authorized, independent, and worth its overhead.
- Ownership includes source, tests, docs, schemas, and generated companions.
- Workers used isolated state and stayed within manifests.
- Results contain reproducible RED, commit, test, path, and conflict evidence.
- Integration order and wave-level validation are deterministic.
- Workers/worktrees are cleaned only after proof-backed completion.

## Examples

**User says:** `/swarm` for three disjoint accepted slices.

Show the ownership matrix, dispatch bounded workers, validate file-backed
results, integrate in order, and independently land each slice.

## Troubleshooting

| Problem | Response |
|---|---|
| file ownership overlaps | serialize or merge the tasks |
| backend unavailable | execute sequentially with the same task/result contract |
| worker leaves scope | reject changes and record a scope escape |
| worker stalls | bounded correction, then close/re-plan |
| worktree cannot reap | retain it until ancestry proof succeeds |

## References

- [swarm.feature](references/swarm.feature) · [execution-steps.md](references/execution-steps.md) · [validation-contract.md](references/validation-contract.md)
- [pre-spawn-friction-gates.md](references/pre-spawn-friction-gates.md) · [shared-checkout-discipline.md](references/shared-checkout-discipline.md) · [worktree-isolation.md](references/worktree-isolation.md)
- [worker-specs.md](references/worker-specs.md) · [worker-pre-task-checks.md](references/worker-pre-task-checks.md) · [worker-pitfalls.md](references/worker-pitfalls.md)
- [conflict-recovery.md](references/conflict-recovery.md) · [scope-escape-template.md](references/scope-escape-template.md) · [cold-start-contexts.md](references/cold-start-contexts.md)
- [backend-codex-subagents.md](references/backend-codex-subagents.md) · [backend-claude-teams.md](references/backend-claude-teams.md) · [backend-background-tasks.md](references/backend-background-tasks.md) · [backend-inline.md](references/backend-inline.md)
- [local-mode.md](references/local-mode.md) · [ol-wave-integration.md](references/ol-wave-integration.md) · [ralph-loop-contract.md](references/ralph-loop-contract.md)
- [agent-genie-coordination-contract.md](references/agent-genie-coordination-contract.md) · [claude-code-latest-features.md](references/claude-code-latest-features.md) · [troubleshooting.md](references/troubleshooting.md)
