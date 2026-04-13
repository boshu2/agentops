---
type: research
date: 2026-04-13
source_bead: na-1f0d
source_epic: ag-amg
---

# Session Intelligence Adoption Readout

## Objective

After the post-merge main run, capture startup fill rate, packet rollout stage,
citation reuse, and feedback. Use that evidence to decide whether Session
Intelligence packet families are ready to broaden into default startup context.

## Plan

- Query packet rollout health for the generic startup path and a
  Session-Intelligence-specific task query.
- Query the 30-day citation report for reuse and feedback signals.
- Compare the result against `cli/cmd/ao/context_packet_status.go` thresholds
  and `docs/contracts/session-intelligence-trust-model.md`.
- Record a replayable decision artifact before consuming the next-work item.

## Pre-Mortem

Scope mode: hold scope.

- Risk: consume the next-work row without explicit proof.
  Mitigation: this readout plus `.agents/releases/evidence-only-closures/na-1f0d.json`
  anchor the decision.
- Risk: treat a healthy startup payload as packet adoption proof.
  Mitigation: separate payload health from packet-family rollout metrics.
- Risk: broaden defaults prematurely.
  Mitigation: apply the trust model and rollout thresholds directly.

Applied prevention checks:

- `.agents/pre-mortem-checks/f-2026-04-03-001.md`: proof-only closures need a
  schema-backed artifact.
- `.agents/pre-mortem-checks/f-2026-04-03-002.md`: next-work consumption needs
  explicit completion proof.

## Inputs

Run at `2026-04-13T04:08:16Z` on `main` at `a0c7785a`.

Commands:

```bash
env -u AGENTOPS_RPI_RUNTIME ao context packet-status --json --limit 10
env -u AGENTOPS_RPI_RUNTIME ao context packet-status --json --phase startup --task "Session Intelligence adoption readout" --limit 10
env -u AGENTOPS_RPI_RUNTIME ao metrics cite-report --json
```

## Packet Rollout

| Signal | Generic startup | Task-specific startup |
|---|---:|---:|
| Payload status | healthy | healthy |
| Payload selected count | 58 | 18 |
| Rollout stage | experimental | experimental |
| Startup fill rate | 1.00 | 1.00 |
| Thin families | 3 of 3 | 3 of 3 |
| Thin ratio | 1.00 | 1.00 |
| Packet reuse artifacts | 0 | 0 |
| Packet reuse sessions | 0 | 0 |
| Packet reuse workspaces | 0 | 0 |

All three experimental packet families are missing in both probes:
`topic-packets`, `source-manifests`, and `promoted-packets`.

## Citation And Feedback

30-day report window:
`2026-03-14T00:07:16.671024-04:00` through
`2026-04-13T00:07:16.671024-04:00`.

| Signal | Value |
|---|---:|
| Total citations | 4,988 |
| Deduped citations | 2,546 |
| Unique artifacts | 87 |
| Unique sessions | 232 |
| Unique workspaces | 1 |
| Hit rate | 0.5402 |
| Hit count | 47 |
| Feedback given | 4,972 of 4,988 |
| Feedback rate | 0.9968 |

Top cited artifacts are existing canonical pattern files:

| Artifact | Count |
|---|---:|
| `.agents/patterns/topological-wave-decomposition.md` | 551 |
| `.agents/patterns/warn-then-fail-ratchet.md` | 549 |
| `.agents/patterns/pre-mortem-first.md` | 521 |
| `.agents/patterns/2026-02-21-council-judges-pattern.md` | 504 |
| `.agents/patterns/cmd-ao-test-hotspot-refactor.md` | 418 |

Interpretation: citation reuse and feedback loops are active, but they are
flowing through canonical pattern artifacts, not experimental packet families.

## Decision

Do not broaden Session Intelligence packet families into default startup
context yet.

The startup payload is healthy, but packet rollout remains experimental. The
readout fails both adoption gates embedded in `packetRolloutStage`:

- Recommended requires `thin_ratio == 0`, at least 5 packet reuse artifacts,
  and startup fill rate of at least 0.75.
- Opt-in requires `thin_ratio <= 0.34`, at least 3 packet reuse artifacts, and
  startup fill rate of at least 0.5.

Observed values are `thin_ratio = 1.00`, `packet_reuse_artifacts = 0`, and
`startup_fill_rate = 1.00`. The trust-model posture still holds: default
startup should prefer canonical findings, rules, risks, and ranked next work;
experimental packet families remain explicit health-gated inputs.

## Follow-Up

No new bead is needed from this readout. The useful decision is negative:
keep Session Intelligence packet families behind the existing health gate until
packet-family artifacts exist and show reuse.
