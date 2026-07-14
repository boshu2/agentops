---
name: goal-design
description: Create deterministic goal-design packets.
---
# Goal Design — Deterministic Intent Packet Authoring

## Codex Lifecycle Guard

When this skill runs in Codex hookless mode (`CODEX_THREAD_ID` is set or
`CODEX_INTERNAL_ORIGINATOR_OVERRIDE` is `Codex Desktop`), run:

```bash
ao codex ensure-start 2>/dev/null || true
```

The CLI records startup once per thread and skips duplicates automatically.

> **Loop position:** optional pre-Discovery adapter.
> Goal Design checks packet shape; it does not judge plan readiness.

## Constraints

- Keep `intent.md` and `driver.md` as the contract of record. A dispatch prompt
  is only a pointer to those artifacts.
- Run the deterministic checker after every edit. A stale digest, schema error,
  unmapped behavior, or identity mismatch blocks handoff.
- Do not invoke Validate, record a semantic verdict, count attempts, manage a
  retry or helper ladder, or decide implementation readiness. Premortem owns the
  one independent semantic verdict after Plan freezes the exact plan.
- Preserve scenario IDs, non-goals, rollback, first failing proof, write scope,
  and close signal when the packet crosses into Discovery or Plan.

## Workflow

1. Shape WHAT before HOW: objective, why, bounded context, non-goals,
   rollback/containment, stale assumptions, and at least one Given/When/Then
   scenario.
2. Create the packet:

   ```bash
   scripts/goal-design-packet.py new <slug> \
     --objective "<goal>" \
     --scenario-name "<observable behavior>" \
     --first-failing-proof "<test or command>" \
     --write-scope "<path or glob>"
   ```

3. If `intent.md` changes, refresh the driver's digest:

   ```bash
   scripts/goal-design-packet.py refresh-digest .agents/goal-design/<slug>
   ```

4. Check the packet deterministically:

   ```bash
   scripts/goal-design-packet.py check .agents/goal-design/<slug>
   ```

   The checker fails closed on stale digest, slug drift, misleading paths,
   unknown scenario IDs, unmapped candidate behavior, and schema violations.

5. Hand the checker-clean packet to Discovery or Plan. Goal Design adds no
   readiness state between the packet and those consumers.
6. For an out-of-session goal API, emit the bounded pointer prompt:

   ```bash
   scripts/goal-design-packet.py prompt .agents/goal-design/<slug>
   ```

## Output Specification

- **Path:** `.agents/goal-design/<slug>/{intent.md,driver.md}`
- **Filename:** exactly `intent.md` and `driver.md`
- **Format:** schema-governed Markdown frontmatter plus concise human-readable
  behavior and candidate tables
- **Validation command:**
  `bats tests/scripts/{goal-design-packet,check-goal-design-packet}.bats`
  and `scripts/check-goal-design-packet.sh <packet-dir>`
- **Downstream handoff:** checker-clean packet path plus preserved scenario IDs
  to Discovery or Plan

## Quality Checklist

- Driver digest matches the current intent bytes.
- Packet identities agree with the directory slug.
- Every candidate maps to a stable scenario, failing proof, write scope, and
  close signal.
- Non-goals and rollback survive the handoff.
- No semantic readiness or loop-governor state appears in the packet.

## Done

Goal Design is done when both files exist, the deterministic checker passes,
and the next action names Discovery, Plan, or a pointer prompt. Semantic plan
judgment happens later, once, in Premortem.
