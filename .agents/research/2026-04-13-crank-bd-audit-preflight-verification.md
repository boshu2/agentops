---
type: research
date: 2026-04-13
source_bead: na-x11m
source_epic: full-session-retro
---

# Crank bd-audit Pre-Flight Verification

## Objective

Verify the harvested `full-session-retro` next-work item:
`Wire bd-audit.sh into crank pre-flight as a blocking gate`.

The requested behavior was a Crank pre-flight warning gate that surfaces
`bd-audit.sh` results before wave execution. If the current repo already
satisfies that contract, consume the stale next-work item with scoped proof
instead of reimplementing it.

## Plan

- Check the shared Crank contract for `bd-audit.sh` pre-flight behavior.
- Check the Codex Crank runtime artifact for equivalent pre-flight guidance.
- Run `scripts/bd-audit.sh --json` to verify the gate has a live, runnable
  audit source.
- Consume only the satisfied next-work item and leave unrelated future work in
  the queue.

## Pre-Mortem

Scope mode: hold scope.

- Risk: reimplement behavior that already landed.
  Mitigation: treat `skills/crank/SKILL.md` and `skills-codex/crank/SKILL.md`
  as the behavior contracts and verify before editing.
- Risk: over-consume the `full-session-retro` queue row.
  Mitigation: mark only the bd-audit pre-flight item consumed; leave the native
  Go command port item available.
- Risk: close a proof-only bead without durable evidence.
  Mitigation: keep this research artifact and an evidence-only closure packet
  in the commit.

Applied prevention checks:

- `.agents/pre-mortem-checks/f-2026-04-03-001.md`: proof-only closures need a
  schema-backed artifact.
- `.agents/pre-mortem-checks/f-2026-04-03-002.md`: next-work consumption needs
  explicit completion proof.

## Evidence

Run on `2026-04-13` from `main`.

Relevant source checks:

```bash
rg -n "full-session-retro|bd-audit.sh|skip-audit|WARNING gate" \
  .agents/rpi/next-work.jsonl \
  skills/crank/SKILL.md \
  skills-codex/crank/SKILL.md \
  docs/CHANGELOG.md
```

Findings:

- `skills/crank/SKILL.md` defines `--skip-audit` as the escape hatch for the
  bd-audit pre-flight gate.
- `skills/crank/SKILL.md` Step 3a.2 runs `scripts/bd-audit.sh --json` before
  wave execution, warns on any flagged beads, and blocks at more than 50%
  flagged.
- `skills-codex/crank/SKILL.md` Step 3a tells Codex Crank to run
  `scripts/bd-audit.sh` before spawning workers and clean up flagged backlog
  hygiene issues before continuing.
- `docs/CHANGELOG.md` records the backlog hygiene gate addition for
  `bd-audit.sh`, `bd-cluster.sh`, and Crank/Codex guidance.

Live audit check:

```bash
env -u AGENTOPS_RPI_RUNTIME bash scripts/bd-audit.sh --json | jq '{summary}'
```

Observed summary:

```json
{
  "summary": {
    "likely_fixed": 1,
    "likely_stale": 0,
    "consolidatable": 0,
    "total": 13,
    "flagged_pct": 7
  }
}
```

The live audit command is runnable and reports a low flagged percentage, so the
current warning/blocking semantics are usable by Crank pre-flight.

## Decision

The harvested item is already satisfied. No Crank implementation change is
needed.

Consume only the `Wire bd-audit.sh into crank pre-flight as a blocking gate`
next-work item with `na-x11m` as the proof bead. Leave the remaining low-priority
`Add bd audit and bd cluster as native Go commands in gastown` item available
for future work because that is a distinct implementation request.
