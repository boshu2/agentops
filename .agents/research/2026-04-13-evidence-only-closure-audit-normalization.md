---
type: research
date: 2026-04-13
source_bead: na-jlmm
source_finding: f-2026-04-03-001
source_epic: dream-findings-router
---

# Evidence-Only Closure Audit Normalization

## Objective

Verify and consume the harvested `dream-findings-router` item
`f-2026-04-03-001`: "Evidence-only maintenance epics need first-class
closure-audit support instead of failing on missing scoped files."

This cycle is a stale-duplicate normalization. It should not modify the audit
implementation unless current source or tests disprove the existing closure
evidence.

## Plan

- Resolve whether a live or closed bead already shipped the audit behavior.
- Check the current closure-integrity source, post-mortem docs, and regression
  coverage for durable evidence-only closure packets.
- Consume only `f-2026-04-03-001` in `.agents/rpi/next-work.jsonl`.
- Emit a durable evidence-only closure packet for this normalization bead.
- Leave `f-2026-04-03-002` untouched because it overlaps the older
  proof-backed next-work suppression claim.

## Pre-Mortem

Scope mode: hold scope.

- Risk: reimplement an already shipped audit path.
  Mitigation: require source, test, and closed-bead proof before deciding this
  is stale.
- Risk: consume the wrong queue item because the router row is a large batch.
  Mitigation: update only the item whose `id` is `f-2026-04-03-001` and leave
  neighboring items unchanged.
- Risk: treat free-text history as completion proof.
  Mitigation: attach a new schema-backed evidence-only closure packet for
  `na-jlmm` and cite concrete source/test paths.
- Risk: accidentally close the adjacent proof-backed suppression finding.
  Mitigation: do not alter `f-2026-04-03-002`.

Applied prevention checks:

- `.agents/findings/f-2026-04-03-001.md`: proof-only closures need a durable
  packet before missing scoped files become audit failures.
- `.agents/findings/f-2026-04-03-002.md`: stale next-work consumption should
  rely on explicit proof, not filename or history heuristics.

## Evidence

Closed bead proof:

```bash
bd show na-uuy.3 --json
```

Result: `na-uuy.3` is closed and records that closure-integrity audits parse
scoped files, distinguish `parser_miss` from `timing_miss`, check
commit/grace/staged/worktree/durable packet evidence, validate evidence-only
closure packets under `.agents/releases/evidence-only-closures`, and passed the
live `na-uuy` audit plus focused BATS coverage.

Source proof:

- `skills/post-mortem/scripts/closure-integrity-audit.sh` validates a durable
  packet with matching `target_id` and artifact list, checks
  `.agents/releases/evidence-only-closures/<id>.json` and
  `.agents/council/evidence-only-closures/<id>.json`, and returns
  `evidence-only-packet` instead of `parser_miss` when no scoped files exist.
- `skills/post-mortem/SKILL.md` requires evidence-only closures to emit
  `.agents/releases/evidence-only-closures/<target-id>.json` and says valid
  durable packets are acceptable audit evidence when no scoped-file section is
  intentional.
- `tests/hooks/lib-hook-helpers.bats` has a regression named
  `closure-integrity-audit.sh: reports durable closure packet in explicit packet
  bucket`.

History proof:

```bash
git log --oneline --all -- skills/post-mortem/scripts/closure-integrity-audit.sh schemas/evidence-only-closure.v1.schema.json tests/hooks/lib-hook-helpers.bats
```

Relevant commits include `d9f23d79 fix(post-mortem): accept evidence-only
closure packets in audit`, `0d9da850 fix(post-mortem): normalize closure packet
evidence mode`, and `bfff7c1b fix(rpi): closure-integrity audit evidence-only
packets and negative-path coverage`.

## Decision

The harvested item is stale. The implementation and regression coverage already
exist on current `main`.

Consume only `f-2026-04-03-001` with `na-jlmm` as the normalization bead and
leave the adjacent `f-2026-04-03-002` finding available for a separate cycle.
