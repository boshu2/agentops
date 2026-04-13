---
type: research
date: 2026-04-13
source_bead: na-xsr2
source_epic: bead-audit-session
---

# ag-yeg Directive Gap Verification

## Objective

Verify the harvested `bead-audit-session` next-work item:
`Close ag-yeg: advance remaining directive gaps (runtime tests, citation gating)`.

The item says directives 1, 2, and 4 still need runtime smoke tests, runtime
install execution tests, and citation-stage gating. Live bd no longer has
`ag-yeg`, so this cycle determines whether the current repo still has those
specific gaps or whether the queue row is stale.

## Plan

- Resolve `ag-yeg` in live bd before acting on the queue row.
- Check `GOALS.md` directive progress for directives 1, 2, and 4.
- Run a bounded `ao goals measure` probe to verify the relevant gates are live.
- Consume only the stale `ag-yeg` next-work item and leave unrelated queue rows
  available.

## Pre-Mortem

Scope mode: hold scope.

- Risk: treat an absent old bead ID as completion proof.
  Mitigation: require current source evidence in `GOALS.md` and a bounded goal
  measurement probe.
- Risk: over-claim external runtime coverage.
  Mitigation: preserve the current GOALS distinction between local CI proof and
  externally gated live runtime or clean-install execution.
- Risk: hide a real `ao goals measure` issue.
  Mitigation: record that the default measurement path is heavy, but the bounded
  command completes and reports the relevant gates.
- Risk: over-consume the `bead-audit-session` row.
  Mitigation: consume only the `ag-yeg` directive-gap item.

Applied prevention checks:

- `.agents/pre-mortem-checks/f-2026-04-03-001.md`: proof-only closures need a
  schema-backed artifact.
- `.agents/pre-mortem-checks/f-2026-04-03-002.md`: next-work consumption needs
  explicit completion proof.

## Evidence

Live bd resolution:

```bash
bd show ag-yeg --json
```

Result: no issue found matching `ag-yeg`.

Source proof in `GOALS.md`:

- Directive 1 records one cross-runtime test, three additional runtime smoke
  tests active in CI through `tests/smoke-test.sh`, and the headless runtime
  validator contract active in CI. Its remaining gap is live hosted-runtime
  execution and inventory proof, which requires external CLIs/auth beyond
  GitHub-hosted runners.
- Directive 2 records the `install-smoke` gate, CI coverage, runtime command
  registration checks when local `cli/bin/ao` exists, and a remaining clean
  install execution gap documented as out of scope for the local gate.
- Directive 4 records `flywheel-lifecycle` as active in CI and tracing capture,
  retrieval, inject, round-trip, and citation. Citation checks are soft-fail on
  sparse corpus and hard-fail when populated corpus lacks citation structure.
- The gate table wires `install-smoke` and `flywheel-lifecycle`.

Bounded goal measurement:

```bash
timeout 30s env -u AGENTOPS_RPI_RUNTIME ./cli/bin/ao goals measure --json --timeout 1 |
  jq '{summary, goals: [.goals[] | select(.goal_id=="install-smoke" or .goal_id=="flywheel-lifecycle") | {goal_id,result,duration_s,output}]}'
```

Observed summary:

```json
{
  "summary": {
    "total": 19,
    "failing": 0,
    "score": 100
  },
  "goals": [
    {
      "goal_id": "install-smoke",
      "result": "pass"
    },
    {
      "goal_id": "flywheel-lifecycle",
      "result": "pass"
    }
  ]
}
```

The default `ao goals measure --json` path can run for longer than a quick
probe because several GOALS.md gates intentionally build/test the repo with
larger per-gate timeouts. That is not evidence that the directive-gap queue item
still needs implementation; the bounded probe confirms the relevant current
gates are wired and passing.

## Decision

The harvested `ag-yeg` directive-gap item is stale.

No code change is needed for the originally listed gaps:

- Runtime smoke coverage exists locally and in CI; the remaining live-hosted
  runtime proof is explicitly external to GitHub-hosted runners.
- Install smoke coverage exists locally and in CI; the remaining clean install
  execution is explicitly external to the local gate.
- Citation-stage gating exists through `flywheel-lifecycle`.

Consume only the `Close ag-yeg: advance remaining directive gaps` queue item
with `na-xsr2` as proof. Leave unrelated remaining next-work rows available.
