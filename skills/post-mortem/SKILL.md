---
name: post-mortem
description: 'Review completed work and learn. Use when: a task, PR arc, or session is finished and you want to extract learnings, or after ≥5 PRs (the scope checkpoint).'
practices:
- dora-metrics
- sre
- lean-startup
hexagonal_role: domain
consumes:
- implement
- validate
- council
produces:
- result.json
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
metadata:
  tier: judgment
  dependencies:
  - council
  - beads-br
  - operationalize
  - toil-mining
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
output_contract: skills/council/schemas/verdict.json
---
# Post-Mortem Skill

Close Move 7 of the operating loop: prove what shipped, extract only reusable
learning, harvest concrete follow-up work, and ratchet the weakest durable
surface that changes future behavior. Execute the workflow; do not only
describe it.

## Critical Constraints

- **Why: no verdict means not done.** Validate the delivered behavior and all
  four closure surfaces—code, documentation, examples, and proof—before learning.
- **Why: avoid phantom closure.** Resolve child evidence in commit → staged →
  worktree order and record evidence-only closure as a durable proof packet.
- **Why: preserve causality.** Compare the original intent, delivered scope,
  prior predictions, and observed outcome; do not invent lessons from hindsight.
- **Why: avoid knowledge landfill.** Promote only a repeated or behavior-changing
  lesson; most observations should remain in the report or die at handoff.
- **Why: prevent rediscovery.** A reusable defect becomes a structured finding,
  compiled prevention context, or a mechanical gate at the appropriate strength.
- **Why: keep work actionable.** Harvest schema-valid next work through its
  available → claimed → consumed lifecycle; never mark an item consumed at pick time.
- **Why: keep closure honest.** Report partial councils, missing artifacts, empty
  harvests, and real-data no-effect results explicitly instead of manufacturing PASS.

## Modes and Inputs

`/post-mortem [target]` wraps a closed bead, epic, PR outcome, or recent session.

| Mode | Behavior |
|---|---|
| `--quick "insight"` | Capture one provisional learning; no council or backlog processing |
| `--scope=pr <num>` | Learn from a merged, rejected, or changes-requested PR |
| `--process-only` | Run backlog processing, activation, retirement, and harvest only |
| `--skip-activate` | Extract and process without promotion |
| `--deep`, `--mixed`, `--debate`, `--explorers=N` | Increase review diversity or depth |
| `--skip-sweep` | Skip Step 2.6 deep-audit sweep |
| `--compound` | Compare two or more goal-measure iterations |

Read [quick-mode.md](references/quick-mode.md),
[pr-scope.md](references/pr-scope.md), or
[compound-engineering-retro.md](references/compound-engineering-retro.md) only
when its mode is selected.

## Workflow

1. **Preflight.** Confirm the repository, completed work, target, and closed
   children. Load checkpoint policy, metadata verification, closure integrity,
   and four-surface closure. Run `scripts/preflight-refs.sh --strict`; block on a
   prior FAIL unless the operator explicitly selects the documented skip.
2. **Reconstruct intent and delivery.** Load the bead/spec, recent commits,
   implementation summary, and plan. Compare planned issues/files/tests with
   delivered evidence. Load `.agents/planning-rules/*.md` and
   `.agents/pre-mortem-checks/*.md` before falling back to
   `.agents/findings/registry.jsonl`.
3. **Audit closure.** Follow [closure-integrity-audit.md](references/closure-integrity-audit.md)
   and [metadata-verification.md](references/metadata-verification.md). For
   evidence-only closure, run `scripts/write-evidence-only-closure.sh` and keep
   the tracked packet at `.agents/releases/evidence-only-closures/<target-id>.json`.
4. **Step 2.6 deep-audit sweep.** Unless `--quick` or `--skip-sweep`, inspect all
   changed files with the validate deep-audit checklist and place the merged
   manifest in `.agents/council/sweep-manifest.md` for adjudication.
5. **Judge the outcome.** Run council with the retrospective preset and three
   perspectives: plan compliance, technical debt, and learnings. Include intent,
   scope delta, closure integrity, metadata failures, prevention context, and
   prediction accuracy. Partial results remain partial; never inflate them.
6. **Extract and ratchet.** Follow [phase-2-extract.md](references/phase-2-extract.md).
   Write reusable findings with `dedup_key` and the tracked registry fields,
   atomically update `.agents/findings/registry.jsonl`, then run
   `bash hooks/finding-compiler.sh --quiet` when present. Route a lesson to the
   weakest sufficient surface: report, learning, skill, always-on doctrine, or gate.
7. **Maintain and harvest.** Process, activate, retire, and harvest via
   [maintenance-phases.md](references/maintenance-phases.md). Append one schema
   v1.4 batch to `.agents/rpi/next-work.jsonl`, validate it, and follow the
   [claim/finalize lifecycle](references/harvest-next-work.md). Update
   `.agents/ao/last-processed` last. Report the highest-priority next route or
   `Flywheel stable — no follow-up items identified.`

### ACT: Harvest Follow-Up Work

#### Step ACT.3: Feed Next-Work

Validate each batch against the tracked
[`docs/contracts/next-work.schema.md`](../../docs/contracts/next-work.schema.md)
and the JSON Schemas linked from the harvest reference. Preserve a known proof
surface in `"proof_ref"`; an empty `items` array is valid when nothing is actionable.

```yaml
source_epic: <target-or-recent>
timestamp: <ISO-8601>
items:
  - title: <actionable follow-up>
    type: task
    severity: medium
    source: post-mortem-finding
    description: <work required>
    evidence: <finding evidence>
    target_repo: <repo>
    "proof_ref": {kind: execution_packet, path: <proof-path>}
    consumed: false
    claim_status: "available"
consumed: false
claim_status: "available"
claimed_by: null
claimed_at: null
consumed_by: null
consumed_at: null
```

#### Step ACT.4: Update Marker

After `bash scripts/validate-next-work.sh --strict` and queue append succeed, update
`.agents/ao/last-processed`. This is the final Phase 4 mutation.

For a normal Codex closeout after artifacts are written, run:

```bash
ao session close --auto-extract
ao flywheel close-loop --quiet
```

Use `ao forge transcript <path-or-glob> --queue` first only when transcript
discovery must be explicit.

## Promotion Ladder

| Surface | Use when |
|---|---|
| report/handoff | one-off observation or context for this closure |
| `.agents/learnings/` | repeated repo-specific learning worth retrieval |
| `SKILL.md` | contextual judgment should fire on a trigger |
| `AGENTS.md` | doctrine applies to most turns |
| gate/check | the behavior must never regress |

When a gate caught a defect that green tests missed, add the review dimension to
`docs/gate/findings-ledger.md`. If the existing gate already catches it, teach
the workflow to run that gate earlier instead of duplicating enforcement.

## Output Specification

- **Artifact directory:** write the human report to
  `.agents/council/YYYY-MM-DD-post-mortem-<topic>.md`; machine closure is
  `result.json`, with optional learning, finding, evidence-only, and next-work artifacts.
- **Filename convention:** ISO date plus a stable topic slug; evidence-only
  packets use `<target-id>.json` and next work remains JSONL.
- **Serialization/schema format:** Markdown report plus schema-valid council
  verdict/result JSON, evidence-only-closure v1 JSON, and next-work v1.4 JSONL.
- **Validator command:** run `bash skills/post-mortem/scripts/validate.sh`,
  `bash scripts/validate-next-work.sh --strict .agents/rpi/next-work.jsonl` when
  harvested, and the evidence writer's schema validation when applicable.
- **Downstream handoff:** consumed by the closed bead, `/rpi`/`/plan`, the
  findings compiler, retrieval, and the next operating-loop turn.

## Quality Rubric

- **Evidence-bound:** every verdict and lesson cites delivered proof or measured outcome.
- **Intent-aware:** planned versus delivered scope and test levels are reconciled.
- **Four-surface complete:** code, docs, examples, and proof are each resolved.
- **Selective:** only reusable learning survives; empty harvests are legitimate.
- **Machine-checkable:** findings, proof packets, and next work validate to schema.
- **Loop-closing:** the next turn can consume the promoted rule or harvested item.

## Examples

**User says:** `/post-mortem age-123`

Audit a completed epic, judge it, ratchet learning,
  and harvest the next work.

**User says:** `/post-mortem --quick "ledger appends must rebase from the current chain tip"`

Write one provisional learning and stop. For `/post-mortem --scope=pr 42`, mine
the PR outcome before normal maintenance.

## Troubleshooting

| Problem | Response |
|---|---|
| Council times out | Report partial evidence and split the review scope |
| Prior checkpoint is FAIL | Repair it or use the explicit skip with rationale |
| No next work exists | Emit an empty valid batch and report flywheel stable |
| Plan and delivered files differ | Record the delta in metadata failures and council context |

## References

- [execution-steps.md](references/execution-steps.md) · [context-gathering.md](references/context-gathering.md) · [plan-compliance-checklist.md](references/plan-compliance-checklist.md)
- [checkpoint-policy.md](references/checkpoint-policy.md) · [closure-integrity-audit.md](references/closure-integrity-audit.md) · [metadata-verification.md](references/metadata-verification.md) · [four-surface-closure.md](references/four-surface-closure.md)
- [phase-2-extract.md](references/phase-2-extract.md) · [learning-templates.md](references/learning-templates.md) · [prediction-tracking.md](references/prediction-tracking.md)
- [backlog-processing.md](references/backlog-processing.md) · [activation-policy.md](references/activation-policy.md) · [maintenance-phases.md](references/maintenance-phases.md)
- [harvest-next-work.md](references/harvest-next-work.md) · [output-templates.md](references/output-templates.md) · [user-reporting.md](references/user-reporting.md)
- [quick-mode.md](references/quick-mode.md) · [pr-scope.md](references/pr-scope.md) · [compound-engineering-retro.md](references/compound-engineering-retro.md)
- [security-patterns.md](references/security-patterns.md) · [retro-history.md](references/retro-history.md) · [streak-tracking.md](references/streak-tracking.md)
- [post-mortem.feature](references/post-mortem.feature) · [pr-retro.feature](references/pr-retro.feature)
