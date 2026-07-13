---
name: postmortem
description: 'Test an explicit retrospective causal question against evidence and counterfactuals after Validate and Learn; never repeat acceptance validation or own general learning bookkeeping. Triggers: "postmortem", "retrospective causal analysis".'
practices:
- sre
- lean-startup
hexagonal_role: domain
consumes:
- learn
- toil-mining
produces:
- postmortem-report.md
context_rel:
- kind: customer-of
  with: learn
- kind: customer-of
  with: toil-mining
skill_api_version: 1
user-invocable: true
metadata:
  tier: judgment
  dependencies:
  - council
  - toil-mining
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
output_contract: skills/postmortem/references/postmortem.feature
---

# Postmortem

> **Purpose:** Answer an explicit retrospective causal question using the
> already-validated outcome and evidence.

## Critical Constraints

- **Why: keep the question causal.** Postmortem is retrospective causal analysis, not the general learning
  umbrella and not a completion gate.
- **Why: avoid duplicate proof.** It consumes an immutable Validate verdict plus Learn receipt and does not re-run acceptance validation by default.
- **Why: expose uncertainty.** Treat causal statements as hypotheses. Separate observed sequence,
  contributing conditions, counterfactuals, and unknowns.
- **Why: prevent causal overclaiming.** A correlation is not promoted to cause without evidence that discriminates
  plausible alternatives.
- **Why: preserve authority boundaries.** Do not rewrite proof, operate delivery or tracker state, change the remaining
  plan, or promote a rule. Return evidence to the caller.
- **Why: keep the record honest.** Empty or inconclusive analysis is valid; manufacture neither certainty nor a
  lesson to make the retrospective feel useful.

## Workflow

1. Pin the verdict, Learn receipt, delivered artifact, and explicit causal
   question.
2. Reconstruct the evidence-backed timeline without importing hidden author
   reasoning as fact.
3. List candidate contributing conditions and at least one plausible
   alternative explanation.
4. Test each claim against cited evidence and a counterfactual: what should
   differ if the claim were false?
5. Optionally use independent judges to challenge contested causal claims.
6. Emit a report containing supported claims, rejected claims, unknowns,
   evidence references, and suggested experiments. Stop.

## Compatibility references

Postmortem no longer owns general closure, promotion, maintenance, or learning
bookkeeping. The following retained paths document that retired umbrella and
remain discoverable only for live compatibility links and migration; do not
load them as the current Postmortem contract:

[activation-policy.md](references/activation-policy.md),
[backlog-processing.md](references/backlog-processing.md),
[checkpoint-policy.md](references/checkpoint-policy.md),
[closure-integrity-audit.md](references/closure-integrity-audit.md),
[compound-engineering-retro.md](references/compound-engineering-retro.md),
[context-gathering.md](references/context-gathering.md),
[execution-steps.md](references/execution-steps.md),
[four-surface-closure.md](references/four-surface-closure.md),
[harvest-next-work.md](references/harvest-next-work.md),
[learning-templates.md](references/learning-templates.md),
[maintenance-phases.md](references/maintenance-phases.md),
[metadata-verification.md](references/metadata-verification.md),
[output-templates.md](references/output-templates.md),
[phase-2-extract.md](references/phase-2-extract.md),
[plan-compliance-checklist.md](references/plan-compliance-checklist.md),
[pr-retro.feature](references/pr-retro.feature),
[pr-scope.md](references/pr-scope.md),
[prediction-tracking.md](references/prediction-tracking.md),
[quick-mode.md](references/quick-mode.md),
[retro-history.md](references/retro-history.md),
[security-patterns.md](references/security-patterns.md),
[streak-tracking.md](references/streak-tracking.md), and
[user-reporting.md](references/user-reporting.md).

## Output Specification

- **Artifact directory:** `.agents/council/`.
- **Filename convention:** `YYYY-MM-DD-postmortem-<topic>.md`.
- **Serialization/schema format:** Markdown with the required sections below;
  behavioral conformance is defined by `references/postmortem.feature`.
- **Required sections:** causal question, pinned inputs, timeline, hypotheses,
  evidence, counterfactuals, unknowns, and experiments.
- **Validator command:** `bash skills/postmortem/scripts/validate.sh`.
- **Downstream handoff:** Learn or the orchestrator may consume the analysis; they own
  any bookkeeping, promotion, planning, or delivery decision.

## Quality Checklist

- [ ] The causal question and all inputs are pinned.
- [ ] Every supported or rejected claim cites evidence and a counterfactual.
- [ ] Alternatives and unknowns remain explicit.
- [ ] The report performs no validation, promotion, planning, or delivery work.

Executable behavior is in [postmortem.feature](references/postmortem.feature).
