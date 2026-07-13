---
name: postmortem
description: Test an explicit retrospective causal
---
# Postmortem

> **Purpose:** Answer an explicit retrospective causal question using the
> already-validated outcome and evidence.

## Critical Constraints

- Postmortem is retrospective causal analysis, not the general learning
  umbrella and not a completion gate.
- It consumes an immutable Validate verdict plus Learn receipt and does not re-run acceptance validation by default.
- Treat causal statements as hypotheses. Separate observed sequence,
  contributing conditions, counterfactuals, and unknowns.
- A correlation is not promoted to cause without evidence that discriminates
  plausible alternatives.
- Do not rewrite proof, operate delivery or tracker state, change the remaining
  plan, or promote a rule. Return evidence to the caller.
- Empty or inconclusive analysis is valid; manufacture neither certainty nor a
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

## Output Specification

- **Artifact:** `.agents/council/YYYY-MM-DD-postmortem-<topic>.md`.
- **Required sections:** causal question, pinned inputs, timeline, hypotheses,
  evidence, counterfactuals, unknowns, and experiments.
- **Validator:** `bash skills/postmortem/scripts/validate.sh`.
- **Downstream:** Learn or the orchestrator may consume the analysis; they own
  any bookkeeping, promotion, planning, or delivery decision.

Executable behavior is in [postmortem.feature](references/postmortem.feature).
