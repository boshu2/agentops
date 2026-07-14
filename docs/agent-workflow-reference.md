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

This repository's current delivery policy is direct push to `main` after
deterministic checks (`ao gate check --fast`). That is repository policy, not a
lifecycle rule. Discovery, Crank, Validate, and Learn end at evidence and
receipts. Delivery may cite that proof; it does not require another LLM landing
verdict. Keep each change a coherent, independently revertible bead arc. On
remote rejection, update against the moving target and retry the selected
delivery command.

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

The private `br` ledger is repository tracking truth (`_beads/issues.jsonl` plus
a local SQLite cache; `br sync` never runs git). It lives in the canonical
checkout, not in linked worktrees. Resolve with `ao beads dir` and invoke as
`BEADS_DIR="$(ao beads dir)" br <cmd>`. For write commands (`create` / `update` /
`close` / `dep`), fail closed: `BEADS_DIR="$(ao beads dir --require)" && export
BEADS_DIR && br …` (`--require` exits non-zero, printing nothing, unless the
resolved directory holds a real ledger). An empty or wrong `BEADS_DIR` lets br
silently write the wrong tracker.

Never add `_beads/`, `.beads/`, or repo-root `.agents/` runtime artifacts to the
public repository. Sync the private nested ledger with
`git -C "$(ao beads dir)" push` (remote `boshu2/agentops-beads`). `bd`/Dolt is
the Gas City substrate store, a different layer; do not use it for this
repository's tracking. Triage the graph with `bv --robot-insights`,
`--robot-plan`, `--robot-priority`.

## Pull exactly one leaf

Claim one ready BDD-shaped leaf only after its prerequisites, admitted base,
acceptance, first RED or baseline, exact write scope, read-only consumers,
rollback, and proof boundary are known. No bead, no push. Free-text acceptance
is invalid: promote to a `.feature` file (canonical when present) or an embedded
`## Scenarios` block before work begins. Default one coherent arc per push;
split scenarios with independent rollback. Carve-out: `type=chore` with
`#trivial` for tiny work.

```bash
BEADS_DIR="$(ao beads dir)" br ready --json
BEADS_DIR="$(ao beads dir)" br update <bead-id> --claim --json
# new work:
BEADS_DIR="$(ao beads dir)" br create "Title" -t task -p 2 --body "..." --json
git worktree add /absolute/path/to/worktree \
  -b <type>/<bead-id>-<scenario-token>-<short-slug> origin/main
```

| Element | Format |
|---|---|
| Branch | `<type>/<bead-id>-<scenario-token>-<short-slug>` · ≤80 chars · full scenario slug if it fits, else `<slug-prefix>-<hash8>` |
| Commit title | `<type>(<scope>): <subject> (<bead-id>)` |
| Required evidence | bead id in commit message or close reason · local gate output path or summary · bounded context when relevant |

The branch name is descriptive provenance, not authority. Confirm the worktree's
HEAD equals the admitted base before editing. One writer owns one active leaf.
Goal and epic records remain aggregate demand and do not occupy WIP.

The host canonical checkout is contended. Agents do not edit it directly: use a
linked worktree for every change (runtime constraints:
[`repo-execution-profile.md`](contracts/repo-execution-profile.md)). Foreign uncommitted files are
quarantined: identify the owner, attach them to a bead, and move them into a
worktree. Keep the canonical root clean and attached to `main`. Run
`bash scripts/check-worktree-disposition.sh` before push and session close.

If another active lane may touch an owned path, stop before mutation. Concurrent
writers require disjoint scopes and separate worktrees; otherwise serialize the
leaves. In an explicitly coordinated workflow, reserve a potentially shared path
before either writer edits it.

## Build within the admitted scope

Create and run the named acceptance check first. Preserve the output that proves
right-reason RED against the admitted base, then make the smallest change that
turns it green. Run focused checks selected from the changed package, document,
contract, or script surface. Fail-fast before land:
`ao gate check --fast --scope head`.

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
repository, not AgentOps, selects the adapter: direct push, pull request, hosted
CI, a dedicated merger, or a cloud-agent callback are all valid. AgentOps may
record the selected adapter, target ref, and resulting identity, but it does not
perform or authorize the Git mutation. Delivery may check remote divergence,
proof freshness, mapping/overlap, and repository policy; it must not recreate
semantic validation or rerun an unchanged full deterministic suite merely
because delivery began.

This repository's adapter:

```bash
ao gate check --fast --scope head
git fetch origin main
# integrate if needed; rerun the scoped gate when the payload changes
git push origin HEAD:main
```

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
and apply the same exact-identity rule. Close tracker state only after that
confirmation.

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

```bash
BEADS_DIR="$(ao beads dir)" br close <id> --reason "Done" --json
```

## Tracker recipes (br)

Use `br` for all task tracking in this repository. Do not create markdown TODO
lists or parallel trackers. Prefer `--json` for programmatic use. Link discovered
work with `discovered-from` dependencies.

```bash
BEADS_DIR="$(ao beads dir)" br ready --json
BEADS_DIR="$(ao beads dir)" br create "Title" --body "..." -t bug|feature|task|epic|chore -p 0-4 --json
BEADS_DIR="$(ao beads dir)" br create "Found bug" --body "..." -p 1 --deps discovered-from:<parent-id> --json
BEADS_DIR="$(ao beads dir)" br update <id> --claim --json
BEADS_DIR="$(ao beads dir)" br update <id> --priority 1 --json
BEADS_DIR="$(ao beads dir)" br update <id> --acceptance-criteria "..." --design "..."
BEADS_DIR="$(ao beads dir)" br close <id> --reason "Completed" --json
BEADS_DIR="$(ao beads dir)" br dep add <child> <parent>
```

Priorities: `0` critical · `1` high · `2` medium (default) · `3` low · `4` backlog.
Hygiene: `br lint`, `br defer` / `br undefer`, `br stale`, `br orphans`,
`br epic`, `br dep tree <id>`, `br dep cycles`. Agent guide: `br robot-docs guide`.

Auto-sync: each write flushes SQLite to `_beads/issues.jsonl` (disable with
`--no-auto-flush`). Explicit control:
`BEADS_DIR="$(ao beads dir)" br sync --flush-only` / `--import-only` / `--status`.
Remote sync of the private ledger:

```bash
BEADS_DIR="$(ao beads dir)" br sync --flush-only
git -C "$(ao beads dir)" add -A
git -C "$(ao beads dir)" commit -m "tracker: <summary>"
git -C "$(ao beads dir)" push
```

Nothing to export from `--flush-only` is fine; the git push of pending ledger
commits remains mandatory when tracker changes exist.

## Provenance ledger

Source of truth for SDLC provenance is the append-only JSONL at
`docs/provenance/ledger.jsonl` (schema `agentops-sdlc-provenance.v1`). Tracker
fields, notes, and comments are a derived projection: ledger wins on
disagreement. Concurrent writers append events; they never rewrite. Review
verdicts are first-class ledger events.

## Local gates

Routine authority before push:

```bash
ao gate check --fast --scope head
```

Full local release evidence:
`ao gate check --full --workflow-coverage --require-workflow-parity`.
Legacy bash fallback only when documented: `AGENTOPS_GATE_BASH=1`.
Dev bootstrap: `bash scripts/install.sh --dev`. Full local suite:
`scripts/ci-local-release.sh`.

Surface-specific checks when the smart gate is insufficient:

```bash
bash skills/heal-skill/scripts/heal.sh --strict
./tests/docs/validate-doc-release.sh
find . -name "*.sh" -type f -not -path "./.git/*" -print0 | xargs -0 shellcheck --severity=error
git ls-files '*.md' | xargs markdownlint
cd cli && make build && make test
./scripts/check-contract-compatibility.sh
bash scripts/validate-ci-policy-parity.sh
bash scripts/check-worktree-disposition.sh
./scripts/validate-manifests.sh --repo-root .
find skills -type l   # must be empty
bash scripts/validate-headless-runtime-skills.sh
bash scripts/validate-codex-override-coverage.sh
bash scripts/validate-codex-rpi-contract.sh
bash scripts/validate-codex-lifecycle-guards.sh
bash scripts/audit-codex-parity.sh
scripts/test-agentops-contract-canaries.sh
```

User-facing CLI changes: before declaring the product path usable, prove the
installed `ao` matches the just-built binary (same-version binaries can still
differ until `make install`):

```bash
cd cli && make build && cd .. && bash scripts/preflight-uat-binary.sh
```

Local-only; not a CI gate.

## Session closeout (landing the plane)

Work is not complete until `git push` succeeds. When ending a session:

1. File beads for remaining follow-up.
2. Run quality gates if code changed (include installed-binary smoke for
   user-facing CLI changes).
3. Update tracker status for finished and in-progress work.
4. Deliver with repository policy, then sync the private tracker:

```bash
ao gate check --fast --scope head
git fetch origin main
git push origin HEAD:main
BEADS_DIR="$(ao beads dir)" br sync --flush-only
git -C "$(ao beads dir)" add -A \
  && git -C "$(ao beads dir)" commit -m "tracker: <summary>" \
  && git -C "$(ao beads dir)" push   # if tracker changes pending
git status   # must show up to date with origin
```

5. Clean up: clear stashes, prune remote branches, validate worktree disposition.
6. Verify all intended changes are committed and pushed.
7. Hand off context for the next session.

Do not stop before push or defer push to the operator. If push fails, resolve and
retry until it succeeds. Never leave a foreign branch-attached worktree without a
recorded disposition.

## Session arc budget

Coherent-arc governs the shape of one shipped arc; session-scope governs the
count. Default: 2–4 arcs per autonomous session. At ≥5 shipped or in-flight arcs
in one session, stop and run a postmortem before continuing (mandatory `/evolve`
checkpoint: `skills/evolve/references/postmortem-checkpoint.md`). That checkpoint
is a re-plan point: it may refactor, reorder, drop, or add remaining arcs from
what the session taught (`/rpi` Agile Re-Plan Loop).

## Releasing

Is a release due? `scripts/check-release-due.sh` (also via
`scripts/release-cadence-check.sh`) reports commits and days since the last
`vX.Y.Z` tag (defaults 50 commits / 14 days; override `RELEASE_DUE_COMMITS` /
`RELEASE_DUE_DAYS`). Signal only; nothing auto-releases.

1. Validate: routine pre-tag sanity `scripts/ci-local-release.sh --quick`; full
   `scripts/ci-local-release.sh` for the actual tag.
2. Tag and push: `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. GitHub Actions runs GoReleaser (binaries, release, Homebrew tap).
4. Upgrade locally: `brew update && brew upgrade agentops`.

Retag (roll post-tag commits into an existing release):
`scripts/retag-release.sh vX.Y.Z` moves the tag to HEAD, pushes, rebuilds the
GitHub release, updates the Homebrew tap, and upgrades locally.

## Helper extraction ratchet

Every helper or library extraction ships a shrink-only observational ratchet in
the same arc (`scripts/lib/ratchet.sh`: detector function + pinned grandfather
file; example `scripts/check-atomic-write-ratchet.sh`). The ratchet claims
observation, not enforcement. Consolidation without a guard accretes hand-rolled
copies. A ratchet graduates to blocking only via a separately earned precision
detector.

## Rollback and recovery

Before consumers land, discard a failed candidate back to its admitted base and
retain RED/verdict receipts only as non-authoritative evidence. After consumers
land, prefer a compatible roll-forward. A coordinated revert follows declared
reverse dependencies; never revert a provider alone while consumers still depend
on it.

After interruption, reconstruct state from repository HEAD/status, the live bead,
candidate and verdict receipts, Learn, delivery evidence, and remote ref, not from
chat memory. Report one legal next action. Do not bootstrap an orchestration
substrate merely to recover a normal local leaf.

## Further surfaces

For executable command truth, inspect `cli/cmd/ao/` and generated
`cli/docs/COMMANDS.md`. For gate and release policy:
`docs/CI-CD.md`, `docs/contracts/ci-jobs.yaml`,
`docs/runbooks/release-process.md`. Runtime and worktree constraints:
[`repo-execution-profile.md`](contracts/repo-execution-profile.md). Codex artifact parity:
[`codex-skill-api.md`](contracts/codex-skill-api.md). CI job detail:
[`CI-CD.md`](CI-CD.md).
