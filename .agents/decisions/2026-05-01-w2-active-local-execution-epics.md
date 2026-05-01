---
id: decision-2026-05-01-w2-active-local-execution-epics
type: decision
date: 2026-05-01
issue: soc-o6eb.3
status: accepted
---

# W2 Active Local Execution Epics

This decision closes W2 as a routing pass. It selects one next executable issue
for each already-discovered local execution epic without reparenting the
portfolio or implementing downstream epics.

## Selected Next Issues

### soc-8412

Selected next issue: `soc-8412.1` - BCLI-1: Extend command contract and
compatibility map.

Evidence:

- `bd show soc-8412 --json` reports `soc-8412` open with 6 children and 0
  closed children.
- `bd show soc-8412.1 --json` reports `soc-8412.1` open, P1, `wave-1`, and
  parented to `soc-8412`.
- `soc-8412.1` is the contract/test slice. Downstream implementation issues
  such as `soc-8412.2` and `soc-8412.3` depend on the contract being settled.

Decision: start with `soc-8412.1`; do not start `soc-8412.2` until the command
contract and compatibility map are validated.

### soc-b8jo

Selected next issue: `soc-b8jo.1` - Document and install host scheduler for
nightly chain.

Evidence:

- `bd show soc-o6eb.3 --json` notes: "Start from soc-8412 ordered chain,
  soc-b8jo.1 scheduler install, and soc-eh1z/PR-validation toil."
- `bd show soc-b8jo.1 --json` reports `soc-b8jo.1` open, P2, and parented to
  `soc-b8jo`.
- `bd ready --json --limit 0` still shows `soc-6wuw` and `soc-lmoq` as ready
  P1 children, but W1 explicitly selected the scheduler lane for W2 sequencing.

Decision: start with `soc-b8jo.1`; leave `soc-6wuw` and `soc-lmoq` visible but
do not interleave them before the scheduler stop condition.

### soc-eh1z

Selected next issue: `soc-0pzj` - Investigate warn-only eval advisory
regressions from PR #204.

Evidence:

- `bd show soc-eh1z --json` reports `soc-eh1z` open with 5 children and 4
  closed children.
- The only open `soc-eh1z` child in live bd output is `soc-0pzj`.
- `soc-0pzj` notes that related eval determinism work remains `soc-v7s8`, so
  the first task is classification: determine whether PR #204 regressions are
  covered by existing determinism work or require a baseline/update fix.

Decision: start with `soc-0pzj`; do not implement `soc-v7s8` under this epic
unless the investigation produces explicit evidence that it is the right route.

## Validation Commands

### soc-8412

Run after implementing `soc-8412.1`:

```bash
cd dev/personal/dotfiles &&
test -f docs/bushido-box-control-plane.md &&
test -f tests/bushido_cli_contract.bats &&
rg 'bushido status|bushido pipeline' docs/bushido-box-control-plane.md &&
bats tests/bushido_cli_contract.bats
```

### soc-b8jo

Run after implementing `soc-b8jo.1`:

```bash
rg -- '--emit-systemd|agentops-nightly-evolution.timer|nightly kill switch|lock' \
  scripts/nightly-evolution.sh docs/runbooks/nightly-evolution.md \
  tests/scripts/nightly-evolution.bats &&
bats tests/scripts/nightly-evolution.bats
```

### soc-eh1z

Run while investigating `soc-0pzj`:

```bash
scripts/eval-agentops.sh --fast
```

If the PR #204 failure is classified as baseline or eval-policy drift, also
run the focused baseline audit before closing the investigation:

```bash
cd cli &&
env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval baseline-audit \
  --root ../evals/agentops-core \
  --baseline-dir ../.agents/evals/baselines \
  --json | jq -e '.policy_mismatch_count == 0'
```

## Stop Conditions

### soc-8412

Stop when `soc-8412.1` satisfies its contract-doc and fixture-test acceptance,
the validation command above passes, and tracker closeout or stale-scope notes
identify `soc-8412.2` as the next Bushido issue. Do not implement pipeline,
status, Mac proxy, deploy-drift, or final validation children in the same W2
route.

### soc-b8jo

Stop when `soc-b8jo.1` has scheduler documentation or generated helper
templates with locking, kill switch, logs, and no source mutation inside CI,
and `bats tests/scripts/nightly-evolution.bats` passes. If host installation
requires operator action, stop with that operator gate recorded instead of
continuing into `soc-6wuw` or `soc-lmoq`.

### soc-eh1z

Stop when `soc-0pzj` classifies the three PR #204 eval advisory regressions as
one of:

- reproduced local deterministic failure with a validated fix,
- baseline/policy drift with baseline-audit evidence, or
- existing nondeterminism already covered by `soc-v7s8`, with evidence and a
  clear handoff.

Do not treat `scripts/eval-agentops.sh --fast` passing once as closure unless
the CI-only PR #204 failure mode is explained.

## Deferred Local Hygiene

These are visibility-only follow-ups for W2. They are not selected next issues
for `soc-8412`, `soc-b8jo`, or `soc-eh1z` in this decision.

- `soc-73tk`: flywheel close-loop dry-run mutates citation metadata; keep near
  the P0 close-loop cluster.
- `soc-7wwp`: canonical RPI dry-run execution-packet alias bug; keep related to
  `soc-b8jo` but do not execute before `soc-b8jo.1`.
- `soc-w7s2`: stale `ao` binary/path freshness bug; keep related to P0
  close-loop freshness.
- `soc-xn5s`: mixed-case close-loop dedup edge case; keep separate from the P0
  terminal-state replay work.
- `soc-hns4`: local daemon projection snapshot caching; route only after the
  active local epics are drained.
- `soc-v7s8`: eval determinism rerun harness; may become the follow-up route
  from `soc-0pzj`, but is not itself selected here.
- `soc-3wh7`: local bd repo fingerprint and hook drift hygiene; W1 routes it to
  local AgentOps hygiene, but it is outside the explicit W2 selected-epic set.

## Discoveries

- `.agents/plans/2026-05-01-nightly-automation-chain.md`,
  `.agents/plans/2026-05-01-pr-validation-toil-reduction.md`, and the
  `soc-8412` plan path named in bd are not present in this checkout. This does
  not block W2 routing because live bd issue bodies, `docs/runbooks/`, scripts,
  and tests provide enough committed-path evidence for next-issue selection.
- Raw `bd ready --json --limit 0` priority order would select `soc-6wuw` and
  `soc-lmoq` before `soc-b8jo.1`, but `soc-o6eb.3` carries W1's explicit
  scheduler-first route. W2 should preserve that route unless a lead changes
  the issue notes.
- `soc-eh1z` has one open child left. Its first action is classification, not
  immediate eval harness implementation, because `soc-0pzj` may resolve into an
  existing `soc-v7s8` determinism route.
