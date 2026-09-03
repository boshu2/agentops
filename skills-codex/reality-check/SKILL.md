---
name: reality-check
description: 'Compare a claimed state with observable repository evidence and report concrete gaps. Triggers: "reality check", "is this claim actually done", "compare claim to repo".'
---
# Reality Check

Compare an explicit claim with observable evidence. Cite every confirmed or
missing behavior with a file, command result, or artifact. Separate:

- confirmed behavior;
- concrete gap;
- incomplete evidence;
- changed assumptions.

## Prompt

```text
Reality-check this claim: PR #1042 finishes the CLI regen migration,
skills-codex/ generation now runs entirely through `ao gate check`, and
`scripts/regen-all.sh` is deleted. Compare against agentops-wt/train2-c
at HEAD. Cite file-by-file evidence for what's confirmed and what's
still a gap.
```

## It's working if

Observable in the trace, without reading the prose:

- Every finding cites a file, command result, or artifact inside
  `.agents/scratch/reality-check/<run-id>$reality-check-report.json`.
- A completion claim carries a disposition per stated goal, each one of
  `confirmed`, `concrete gap`, or `unverifiable`.
- `skills/reality-check/scripts/validate-output.sh` accepts the report
  with no `verdict`, `readiness`, or `PASS` field present.
- An untestable line is filed as `incomplete-evidence` with the missing
  artifact named.

## Vision-coverage audit

When the claim is a completion or status claim, audit it against the stated
goals, not against what happens to exist. Enumerate every goal in the vision,
plan, or intent source and give each a disposition: confirmed with evidence,
concrete gap, or unverifiable. The audit is complete only when every stated
goal carries a disposition; full coverage of the built surface alone proves
nothing about completion. The named failure mode is built-world bias:
auditing only the code that exists, so goals nobody started never surface as
gaps.

## Frozen question variants

When the same check runs across multiple passes or sessions, freeze the exact
question wording before the first pass and ask it identically in every pass;
record the frozen wording in the report. A pass that answers a reworded
question starts a new baseline — comparing it against earlier passes is the
drifting-rubric failure mode, and its answer does not count as a repeated
measurement.

## Ambition-escalation checkpoint

When invoked during planning, compare the currently planned scope against the
originally stated goal. Planned work that cannot be traced to a stated goal
is reported as an escalation gap, exactly like a missing behavior. Reality
Check reports the escalation; the caller decides whether the ambition or the
stated goal changes.

## Output

- **Artifact directory:** `.agents/scratch/reality-check/<run-id>/`.
- **Filename:** `reality-check-report.json`.
- **Format:** `reality-check-report.v1` JSON — the checked claim, one finding per
  confirmed behavior, concrete gap, incomplete-evidence item, or changed
  assumption (each with cited evidence), and, for a completion or status claim,
  the goal-by-goal coverage disposition. It carries no `verdict`, `readiness`, or
  `PASS` field; the validator rejects one.
- **Validation command:**
  `skills/reality-check/scripts/validate-output.sh <reality-check-report.json>`.

If the claim cannot be tested against any observable evidence, report it as
incomplete-evidence with the missing artifact named — never resolve an
untestable claim as confirmed.

## Boundary

Return the report to the caller. Plan may use concrete gaps to refine the
existing bead or caller intent. Reality Check reports observations; it does not
mint a verdict or `PASS` of any version, create work, schedule, claim,
implement, validate, retry, or deliver.
