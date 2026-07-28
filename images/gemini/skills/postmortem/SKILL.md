---
name: postmortem
description: 'Optionally test a retrospective causal question against durable verdict evidence. Triggers: "postmortem", "causal retrospective", "test a retrospective hypothesis".'
practices:
- sre
- lean-startup
hexagonal_role: domain
consumes:
- verdict.v2
produces:
- postmortem-report.md
context_rel: []
skill_api_version: 1
user-invocable: true
metadata:
  capabilities: [postmortem]
  effects: [write_postmortem_report]
  canonical_status: canonical
  disposition: keep_strategy
  tier: judgment
  dependencies: []
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
output_contract: 'YYYY-MM-DD-postmortem-<topic>.md — markdown causal analysis (causal question, pinned inputs, timeline, hypotheses, counterfactuals, unknowns, experiments)'
---

# Postmortem

> **Purpose:** Answer an explicit retrospective causal question using the
> already-validated outcome and evidence.

## Critical Constraints

- Because proof and causal inference are different judgments, Postmortem is retrospective causal analysis, not the general learning umbrella and not a completion gate.
- It consumes immutable Validate verdict evidence and does not re-run acceptance validation because Validate already owns that proof.
- Treat causal statements as hypotheses because causal confidence must survive
  alternatives. Separate observed sequence, contributing conditions,
  counterfactuals, and unknowns.
- A correlation is not promoted to cause without evidence that discriminates
  plausible alternatives.
- Because the caller owns delivery decisions, do not rewrite proof, operate
  tracker state, change the remaining plan, or promote a rule. Return evidence
  to the caller.
- Empty or inconclusive analysis is valid; manufacture neither certainty nor a
  lesson to make the retrospective feel useful.

## Workflow

1. Pin the verdict, subject evidence, and explicit causal
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

## Correlation-to-cause discrimination

A fix is proven when the mechanism is demonstrated, not when symptoms stop.
Promoting a claim from correlation to cause requires all three:

- a stated mechanism — the specific path by which the condition produced the
  outcome, in terms a reader could check against the subject;
- discriminating evidence — an observation that the mechanism predicts and at
  least one plausible alternative does not;
- a counterfactual test — what should have differed if the claim were false,
  with the cited evidence showing it did differ.

Symptom disappearance after a change satisfies none of these on its own: the
change and the recovery may share an unobserved cause, or the symptom may be
intermittent. The named failure mode is post-hoc fix attribution — "we
changed X and the failure stopped, therefore X was the cause." Claims backed
only by symptom cessation stay in the report as correlations with the
untested alternatives listed, and the suggested experiment is the
discrimination that would settle them. Stop condition: every supported causal
claim in the report carries all three elements with citations; anything less
is filed under correlations or unknowns, never silently promoted.

## Output Specification

- **Artifact directory:** `.agents/scratch/postmortem/`.
- **Filename convention:** `YYYY-MM-DD-postmortem-<topic>.md`.
- **Serialization/schema format:** Markdown with causal question, pinned inputs,
  timeline, hypotheses, evidence, counterfactuals, unknowns, and experiments.
- **Validator command:** `bash skills/postmortem/scripts/validate.sh`.
- **Downstream handoff:** Learn or the caller may consume the analysis; they own
  any bookkeeping, promotion, planning, or delivery decision.

## Quality Checklist

- [ ] The causal question and immutable inputs are pinned.
- [ ] Supported and rejected claims cite discriminating evidence.
- [ ] Alternatives, counterfactuals, and unknowns remain visible.
- [ ] The report stops short of proof, planning, tracker, and delivery authority.

Executable behavior is in [postmortem.feature](references/postmortem.feature).
