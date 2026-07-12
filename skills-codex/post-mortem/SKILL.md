---
name: post-mortem
description: 'Close completed work and ratchet learning. Triggers: "$post-mortem", "review completed work", "harvest learnings".'
---
# $post-mortem — Evidence-Bound Closeout

Treat `skills/post-mortem/SKILL.md` as the canonical close-out contract and `skills-codex/post-mortem/SKILL.md` as the Codex-facing artifact.

Execute the closeout. Prove what shipped, extract only reusable learning, write
machine-checkable follow-up work, and make the next loop able to consume it.

## Critical Constraints

- **Why: no verdict means not done.** Validate code, documentation, examples,
  and proof before extracting learning.
- **Why: avoid phantom closure.** Resolve evidence in commit → staged → worktree
  order; evidence-only closure needs a durable proof packet.
- **Why: preserve causality.** Compare original intent, delivered scope, prior
  predictions, and observed outcomes; do not manufacture hindsight lessons.
- **Why: avoid a knowledge graveyard.** Promote only repeated or behavior-changing
  insights and use the weakest durable surface that changes future behavior.
- **Why: keep the queue truthful.** Harvest valid next work as available, then
  claim, release, or consume it through the documented lifecycle.
- **Why: keep review honest.** Partial councils, missing proof, and empty harvests
  remain explicit WARN/FAIL/stable outcomes.

## Modes

| Invocation | Route |
|---|---|
| `$post-mortem [target]` | Full retrospective closeout |
| `$post-mortem --quick "insight"` | One provisional learning; then stop |
| `$post-mortem --scope=pr <num>` | PR outcome as the closure signal |
| `$post-mortem --process-only` | Maintenance and harvest only |
| `$post-mortem --skip-sweep` | Omit Step 2.6 deep-audit sweep |
| `$post-mortem --compound` | Compare repeated goal-measure iterations |

`--deep`, `--mixed`, `--debate`, and `--explorers=N` increase review depth.

## Workflow

1. **Preflight and load policy.** Resolve the repo tracker, use `br` to inspect
   the target and closed children, then read checkpoint, metadata,
   closure-integrity, and four-surface modules. Run `scripts/preflight-refs.sh
   --strict`; a prior FAIL blocks by default.
2. **Reconstruct the arc.** Load bead/spec, plan, commits, delivered files and
   tests. Prefer `.agents/planning-rules/*.md` and
   `.agents/pre-mortem-checks/*.md`, then `.agents/findings/registry.jsonl`.
3. **Prove closure.** Run closure-integrity and metadata checks. Durable
   evidence-only proof lives at
   `.agents/releases/evidence-only-closures/<target-id>.json`.
4. **Step 2.6 audit sweep.** Unless quick or `--skip-sweep`, inspect changed
   files and merge findings into `.agents/council/sweep-manifest.md`.
5. **Judge.** Use the retrospective council perspectives: plan compliance,
   technical debt, and learnings. Include scope delta, closure evidence,
   prevention context, metadata failures, and prediction accuracy.
6. **Extract and ratchet.** Normalize reusable findings with `dedup_key`, update
   `.agents/findings/registry.jsonl` atomically, and run
   `bash hooks/finding-compiler.sh --quiet` when present. Route each lesson to
   report, learning, skill, doctrine, or gate according to enforcement need.
7. **Maintain and harvest.** Process, activate, retire, and append one valid
   next-work v1.4 batch. Follow the claim/finalize lifecycle; update
   `.agents/ao/last-processed` last. Report the best next route or a stable flywheel.

### ACT: Harvest Follow-Up Work

#### Step ACT.3: Feed Next-Work

Validate the batch against
[`docs/contracts/next-work.schema.md`](../../docs/contracts/next-work.schema.md).
Keep known proof on the item as `"proof_ref"`; `items` may be empty when the
post-mortem finds nothing actionable.

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

Update `.agents/ao/last-processed` only after
`bash scripts/validate-next-work.sh --strict` and append succeed.

After artifacts are written:

```bash
ao session close --auto-extract
ao flywheel close-loop --quiet
```

Use `ao forge transcript <path-or-glob> --queue` first only when transcript
discovery must be explicit.

## Codex Execution Profile

- Keep the council/validation summary concise, then write learnings and harvested work to disk.
- Prefer concrete follow-up items that can flow directly into `.agents/rpi/next-work.jsonl` for the next Codex loop.
- Own Codex closeout during the post-mortem flywheel phase by running `ao session close --auto-extract` followed by `ao flywheel close-loop --quiet`; use `ao forge transcript <path-or-glob> --queue` first only when transcript discovery must be explicit.
- Keep harvested work machine-checkable: available on write, then claim/release/consume through the queue lifecycle.

## Guardrails

- Never replace deterministic closure evidence with a confident narrative.
- Keep report prose concise; persist detailed proof and next work to canonical paths.
- Do not promote a one-off observation into always-on doctrine or a new gate.
- Do not mark harvested work consumed before its implementation succeeds.
- Do not hide missing artifacts, partial reviewers, or real-data no-effect results.

## Output Specification

- **Artifact directory:** `.agents/council/` for the dated report and
  `result.json`; optional learning, finding, proof, and queue artifacts use their
  canonical `.agents/` directories.
- **Filename convention:** `YYYY-MM-DD-post-mortem-<topic>.md`; proof packets use
  `<target-id>.json`; harvested batches stay in `next-work.jsonl`.
- **Serialization/schema format:** Markdown plus council verdict/result JSON,
  evidence-only-closure v1 JSON, and next-work v1.4 JSONL.
- **Validator command:** run `bash skills-codex/post-mortem/scripts/validate.sh`
  and strict next-work/evidence schema validation for artifacts produced.
- **Downstream handoff:** consumed by the closed bead, `$rpi`/`$plan`, compiled
  prevention context, retrieval, and the next Codex loop.

## Quality Rubric

- Evidence and real measurements support every verdict and promoted lesson.
- Planned and delivered scope, tests, and four closure surfaces reconcile.
- Findings, proof packets, and harvested work remain schema-valid.
- Promotion strength matches recurrence and enforcement need.
- The next Codex loop can claim and consume the result without chat context.

## Examples

- `$post-mortem age-123` — close an epic and harvest its next work.
- `$post-mortem --quick "rebase ledger appends from the current chain tip"` —
  record one provisional learning.
- `$post-mortem --scope=pr 42` — mine a PR outcome before normal maintenance.

## Troubleshooting

| Problem | Response |
|---|---|
| Council times out | Report partial evidence or split the review scope |
| Prior checkpoint failed | Repair it or document the explicit skip |
| No follow-up exists | Write an empty valid batch and report flywheel stable |
| Plan and delivery differ | Put the delta in metadata failures and council context |

## References

- [context-gathering.md](references/context-gathering.md) · [plan-compliance-checklist.md](references/plan-compliance-checklist.md)
- [checkpoint-policy.md](references/checkpoint-policy.md) · [closure-integrity-audit.md](references/closure-integrity-audit.md) · [metadata-verification.md](references/metadata-verification.md) · [four-surface-closure.md](references/four-surface-closure.md)
- [learning-templates.md](references/learning-templates.md) · [prediction-tracking.md](references/prediction-tracking.md) · [streak-tracking.md](references/streak-tracking.md)
- [backlog-processing.md](references/backlog-processing.md) · [activation-policy.md](references/activation-policy.md) · [maintenance-phases.md](references/maintenance-phases.md)
- [harvest-next-work.md](references/harvest-next-work.md) · [output-templates.md](references/output-templates.md)
- [security-patterns.md](references/security-patterns.md) · [retro-history.md](references/retro-history.md) · [compound-engineering-retro.md](references/compound-engineering-retro.md)
