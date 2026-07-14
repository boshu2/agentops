# Agent Workflow Reference

> Repository-specific evidence mechanics for the canonical
> [Operating Loop](architecture/operating-loop.md). Read this only for bead,
> worktree, candidate, delivery record, provenance, or closeout operations. The compact
> [`AGENTS.md`](../AGENTS.md) defines the legal transitions; this file explains
> how this repository performs them.

AgentOps validation ends at evidence. Git delivery is a separate repository
transition that consumes that evidence. A direct push, pull request, external CI
pipeline, or another repository's merge queue may replace the delivery adapter
without changing the operating loop.

## Resolve the live repository state

Before a tracked mutation:

```bash
git status --short
git fetch origin main
git rev-parse origin/main
BEADS_DIR="$(ao beads dir --require)" && export BEADS_DIR
br show <bead-id>
```

Record the exact remote-main SHA used for admission. A moving remote is normal;
it is not a reason to restart semantic work. It becomes relevant only when the
candidate is mapped or delivered.

The private `br` ledger is repository tracking truth. It lives outside linked
worktrees and must be resolved explicitly. Never add `_beads/`, `.beads/`, or
repo-root `.agents/` runtime artifacts to the public repository. `bd`/Dolt may be
used by a Gas City substrate, but it is not this repository's tracker.

## Pull exactly one leaf

Claim one ready BDD-shaped leaf only after its prerequisites, admitted base,
acceptance, first RED or baseline, exact write scope, read-only consumers,
rollback, and proof boundary are known.

```bash
br update <bead-id> --claim
git worktree add /absolute/path/to/worktree -b codex/<bead-id> origin/main
```

The branch name is descriptive provenance, not authority. Confirm the worktree's
HEAD equals the admitted base before editing. One writer owns one active leaf.
Goal and epic records remain aggregate demand and do not occupy WIP.

If another active lane may touch an owned path, stop before mutation. Concurrent
writers require disjoint scopes and separate worktrees; otherwise serialize the
leaves. In an explicitly coordinated workflow, reserve a potentially shared path
before either writer edits it.

## Build within the admitted scope

Create and run the named acceptance check first. Preserve the output that proves
right-reason RED against the admitted base, then make the smallest change that
turns it green. Run focused checks selected from the changed package, document,
contract, or script surface.

Before each commit and before handoff:

```bash
git status --short
git diff --check
git diff --name-only
git diff --cached --name-only
```

Every changed path must belong to the leaf. A read-only consumer that needs an
edit returns the leaf to Plan; it is not appended during Crank. Generated outputs
are writable only when the leaf names their source owner and regeneration command.

## Commit and freeze one candidate

Commit the complete intended leaf once its focused deterministic checks pass.
Freeze a candidate receipt containing:

- bead ID and admitted base SHA;
- candidate commit SHA and tree SHA;
- clean/dirty status;
- exact owned path list and blob/deletion identities;
- acceptance claims and commands;
- deterministic results and relevant tool/registry identities;
- author identity.

Example identity capture:

```bash
candidate_sha="$(git rev-parse HEAD)"
candidate_tree="$(git rev-parse HEAD^{tree})"
git status --porcelain=v1
git diff --name-status <admitted-base>..."$candidate_sha"
```

The receipt is immutable. Do not amend or repair the frozen candidate in place.
Any source edit produces a new candidate identity and invalidates the prior
semantic verdict. Local ignored receipts are evidence, not source authority.

## Hand the exact candidate to Validate

Run the leaf's declared deterministic commands once for the frozen input. Then a
fresh validator whose identity differs from the author reviews:

- admitted base and candidate/tree identities;
- exact owned paths;
- acceptance claims and non-goals;
- RED/GREEN and deterministic receipts;
- rollback and relevant read-only consumers.

The validator writes one candidate-bound closure with claim citations, a complete
blocker set, PASS/FAIL, and NOTE/REPAIR/REPLAN dispositions. It does not edit the
candidate. `REPAIR` returns one consolidated batch to the author; `REPLAN` returns
to the earliest invalidated plan move. After a repair, refreeze before any new
verdict.

## Record Learn before delivery

`/learn` consumes the immutable Validate closure and digest. Its minimal receipt
records candidate validity, remaining work, and `plan_impact`. It neither repairs
the candidate nor grants Git authority. If Learn reports material plan impact, the
orchestrator decides whether that invalidates the candidate before delivery.

## Deliver through the repository adapter

Delivery must consume the same candidate, PASS verdict, and Learn receipt. The
repository—not AgentOps—selects the adapter: direct push, pull request, hosted
CI, a dedicated merger, or a cloud-agent callback are all valid. AgentOps may
record the selected adapter, target ref, and resulting identity, but it does not
perform or authorize the Git mutation. Delivery may check remote divergence,
proof freshness, mapping/overlap, and repository policy; it must not recreate
semantic validation or rerun an unchanged full deterministic suite merely
because delivery began.

If `origin/main` moved, compare the candidate's owned blobs/deletions and declared
dependencies with the new base. Byte-identical owned semantics plus green
overlap/mapping evidence may reuse the verdict. A changed owned blob, acceptance
claim, proof dependency, or ambiguous overlap invalidates the affected proof and
returns to the earliest invalidated move.

Do not close the bead because a repository merge or push command exited zero.
The repository adapter verifies the exact remote identity and supplies that
evidence to the report:

```bash
git ls-remote origin refs/heads/main
git merge-base --is-ancestor <delivered-sha> origin/main
```

Repository policy may use another target ref or delivery adapter; record that ref
and apply the same exact-identity rule.

## Report, then release WIP

The terminal tranche report includes:

- `LANDED` candidate/delivery identity;
- `REMOTE VERIFIED` ref and SHA;
- `CLOSED LEAF` tracker result;
- goal/epic status without treating it as WIP;
- next ready leaf, if any;
- residual risk and explicitly unchecked scope.

Only after remote verification and that report may the tracker leaf close and the
writer pull another leaf. Tracker state follows repository evidence; it does not
substitute for it.

## Rollback and recovery

Before consumers land, discard a failed candidate back to its admitted base and
retain RED/verdict receipts only as non-authoritative evidence. After consumers
land, prefer a compatible roll-forward. A coordinated revert follows declared
reverse dependencies; never revert a provider alone while consumers still depend
on it.

After interruption, reconstruct state from repository HEAD/status, the live bead,
candidate and verdict receipts, Learn, delivery evidence, and remote ref—not from
chat memory. Report one legal next action. Do not bootstrap an orchestration
substrate merely to recover a normal local leaf.

## Routed detail retained during documentation migration

The tiered siblings remain read-only compatibility consumers until their migration
leaf proves zero live consumers:

- [`AGENTS-WORKFLOW.md`](../AGENTS-WORKFLOW.md) — legacy expanded workflow detail;
- [`AGENTS-CI.md`](../AGENTS-CI.md) — CI and release backstops;
- [`AGENTS-CODEX.md`](../AGENTS-CODEX.md) — Codex artifact parity;
- [`AGENTS-RUNTIME.md`](../AGENTS-RUNTIME.md) — worktree and runtime constraints.

For executable command truth, inspect `cli/cmd/ao/` and generated
`cli/docs/COMMANDS.md`. For gate/release policy, use `docs/CI-CD.md`,
`docs/contracts/ci-jobs.yaml`, and `docs/runbooks/release-process.md`.
