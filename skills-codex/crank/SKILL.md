---
name: crank
description: 'Execute implementation waves. Triggers: "crank an epic", "execute implementation waves", "drive the bead wave plan".'
---
# $crank — Autonomous Epic Execution (Codex Native)

> **Quick Ref:** Execute every tracker-ready slice through bounded Codex worker waves, independent verification, pawl admission, and landed-only closeout.

**You must execute this workflow. Do not just describe it.**

## Constraints

- Execute only tracker-ready vertical slices because crank consumes an accepted plan; it does not silently redefine intent.
- Parallelize only disjoint write scopes and serialize shared derived surfaces to prevent workers from invalidating one another's base.
- Keep workers out of the acceptance and landing authority because producer claims require a lead-owned gate and an independent pawl verdict.
- Consult the pawl and take one bounded helper pass for ordinary blockers before escalating because human interruption is the terminal recovery path.

## Architecture and Backend

Crank owns epic lifecycle, wave packets, acceptance, tracker synchronization, and closeout. Workers own only their assigned implementation scope. Prefer Codex session agents: `spawn_agent` for a fresh worker or explorer, `wait_agent` for completion, `send_input` for a short correction, and `close_agent` for a stalled or unnecessary lane. Do not fall back to legacy CSV fan-out or host-task polling.

In hookless Codex sessions, initialize startup context once before the first wave:

```bash
ao codex ensure-start 2>/dev/null || true
```

Use `$agent-native` plus `$ntm` only for explicitly requested durable, pane-shaped work. Ordinary crank waves stay on the native Codex agent surface.

## Global Limits and Completion

- `MAX_EPIC_WAVES=50`; reaching the cap returns `BLOCKED`.
- Each task gets at most two adjusted retries across all waves.
- End every run with exactly one marker: `<promise>DONE</promise>`, `<promise>PARTIAL</promise>`, or `<promise>BLOCKED</promise>`.
- `DONE` requires every slice accepted, landed, and closed; a worker's completion message is never sufficient.

## Execution Workflow

Given `$crank [epic-id | .agents/rpi/execution-packet.json | plan-file.md | "description"]`:

### 1. Preflight and target

1. Load applicable context with `ao lookup --query "<epic-title>" --limit 5` when `ao` is available.
2. Resolve beads mode through `ao beads exec` and operate the resolved beads_rust tracker as `BEADS_DIR="$(ao beads dir)" br ...`; otherwise preserve the plan or execution-packet objective without inventing an epic.
3. Refuse to implement on `main`/`master`; follow [branch isolation](references/branch-isolation.md).
4. Initialize `.agents/crank/SHARED_TASK_NOTES.md` and append-only `.agents/rpi/plan-mutations.jsonl` using their reference schemas.
5. Read ready work and reject an empty set unless the epic is already complete.

**Execution-packet/file mode:** read `objective`, `epic_id`, `tracker_mode`, `done_criteria`, and `validation_commands` from `.agents/rpi/execution-packet.json`. Preserve the packet objective across retries; when `epic_id` is absent, continue file-backed instead of inventing tracker identity.

**Checkpoint:** verify before dispatch that each slice has an executable acceptance command, `metadata.issue_type`, an explicit file manifest, and no collision with another lane.

### 2. Assemble the wave

Apply FIRE (Find → Ignite → Reap → Escalate) for each wave. Build `.agents/crank/wave-<N>-tasks.json`; each task records issue ID, subject, acceptance criteria, allowed files, validation command, test level, and `metadata.issue_type`. Display the ownership table before spawning.

Two slices collide when they write the same path or regenerate the same derived surface, including registries, context maps, CLI projections, and Codex manifests. Split collisions into sequential sub-waves based on the freshly landed prior commit. For two or more disjoint writers, follow [parallel-wave isolation](references/parallel-wave-isolation.md).

Use `--test-first` to order SPEC → TEST → RED checkpoint → GREEN implementation. A refactor-under-green wave changes no test.

### 3. Dispatch Codex workers

Spawn one `worker` per implementation slice. Spawn a short-lived `explorer` first when the file manifest is unknown. Every worker prompt includes:

- the exact issue and acceptance contract;
- the allowed-file manifest and validation command;
- relevant shared notes and language standards;
- a prohibition on landing, closing, or editing outside its scope;
- a request for concise durable results plus a `Discoveries` section.

Use the current runtime's legal native primitives only. Read [worker specs](references/worker-specs.md), [team coordination](references/team-coordination.md), and the [Ralph loop contract](references/ralph-loop-contract.md) when assembling prompts.

### 4. Verify and checkpoint

After `wait_agent`, the lead—not the worker—must:

1. Read the actual diff and compare it with the slice claim and file manifest.
2. Run the slice acceptance command and any wave-level integration checks.
3. Run CI-policy parity when `.github/workflows/*.yml` changed.
4. Record PASS, FAIL, or BLOCKED in the tracker without closing unlanded work.
5. Write `.agents/crank/wave-<N>-checkpoint.json` and validate it with `bash skills-codex/crank/scripts/validate-wave-checkpoint.sh <checkpoint>`.
6. Harvest durable discoveries and append every plan mutation with its before/after reason.
7. Make the lead-only commit after all owned results pass.

**Checkpoint:** do not consume or hand off a wave checkpoint until its schema, live `git_sha`, timestamp, file list, and acceptance verdict validate.

### 5. Recover through the pawl

Classify a failed task as `RETRY` (transient, adjusted, max two), `DECOMPOSE` (replace it with smaller tracked slices), or `PRUNE` (remove it with the surviving block reason). Log `task_removed`/`task_added` mutations for decomposition and `task_removed` for pruning.

On an ordinary blocker, consult the pawl and take one bounded fresh-context helper pass for that blocker class. Resume on `UNSTUCK`; escalate only on `ESCALATE`, explicit judgment/refusal, exhausted budget, or a surviving block after the helper. Read [failure recovery](references/failure-recovery.md) and [failure taxonomy](references/failure-taxonomy.md).

### 6. Gate, land, and close

For each bead, in its own worktree and serialized against hot `main`:

1. Run `ao gate check --fast --scope head`.
2. Obtain independent `$pawl-review` evidence and a live-head `CONFIRMED` verdict from `ao pawl`; `REFUTED` returns to implementation.
3. Run `bash scripts/pawl-land.sh <bead>`; stale heads rebase feature-only, prove identity, rerun the gate, and receive exact-new-SHA confirmation.
4. Close only after `git merge-base --is-ancestor <feature-sha> origin/main` succeeds.
5. Close an epic only after every child is landed and closed.

Use [land protocol](references/land-protocol.md) for multi-lane serialization and the external-PR variant. A closed bead is a sensor: record the learning, and re-plan only when it falsifies an assumption used by the remaining DAG.

### 7. Loop and finish

Return to ready work until none remains or the wave/retry budget stops progress. Run `$validate` over the completed epic, archive shared notes, report issue count and iterations, then emit the truthful completion marker. Feed surprising wave evidence up to `$rpi` so it can re-plan the remaining route instead of blindly retrying it.

## Output Specification

- **Path:** committed slice changes, wave packets/checkpoints under `.agents/crank/`, worker evidence under `.agents/crank/results/`, and tracker state in the resolved ledger.
- **Filename:** `wave-<N>-tasks.json`, `wave-<N>-checkpoint.json`, `SHARED_TASK_NOTES.md`, and `plan-mutations.jsonl`; preserve worker result filenames declared in their packets.
- **Format:** schema-valid JSON/JSONL evidence plus a markdown closeout containing epic ID/title, completed count, iterations, validation result, and one promise marker.
- **Exit code:** run `bash skills-codex/crank/scripts/validate.sh` and require zero; validate every checkpoint with `validate-wave-checkpoint.sh`; any failed acceptance or malformed evidence prevents `DONE`.
- **Downstream handoff:** pass committed slices and validated evidence to `$validate`, then the pawl; only landed ancestry permits tracker close.

## Quality Checklist

- Every completed slice has RED/GREEN acceptance evidence, owned files, and tracker state consistent with its landed commit.
- Parallel waves contain no shared write or generated-surface collision, and sequential dependencies use the freshly landed prior base.
- Recovery consults the pawl before the human and preserves the one-helper-pass budget per blocker class.
- The final marker matches reality: no `DONE` while issues, failed checks, unlanded commits, or unresolved pawl findings remain.

## Reference Documents

- Core flow: [fire](references/fire.md), [wave patterns](references/wave-patterns.md), [test-first mode](references/test-first-mode.md), [UAT integration](references/uat-integration-wave.md), and [wave-1 consistency](references/wave1-spec-consistency-checklist.md).
- Isolation and dispatch: [branch isolation](references/branch-isolation.md), [parallel isolation](references/parallel-wave-isolation.md), [worktree per worker](references/worktree-per-worker.md), [GC pool dispatch](references/gc-pool-dispatch.md), and [task examples](references/taskcreate-examples.md).
- Contracts and coordination: [contract template](references/contract-template.md), [worker specs](references/worker-specs.md), [team coordination](references/team-coordination.md), [Ralph loop](references/ralph-loop-contract.md), and [shared notes](references/shared-task-notes.md).
- Evidence and landing: [external gate](references/external-gate-protocol.md), [plan mutations](references/plan-mutations.md), [commit strategies](references/commit-strategies.md), and [land protocol](references/land-protocol.md).
- Recovery: [failure recovery](references/failure-recovery.md), [failure taxonomy](references/failure-taxonomy.md), [de-sloppify](references/de-sloppify.md), [troubleshooting](references/troubleshooting.md), and [ship-loop anti-patterns](references/ship-loop-anti-patterns.md).
