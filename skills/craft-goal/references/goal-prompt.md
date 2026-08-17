# Mayor-style goal prompt

Copy this prompt verbatim, replacing every angle-bracket field. Do not delete
the wave, hard-envelope, and terminal-report sections.

```text
Goal outcome:
<larger caller-visible result>

Terminal acceptance and evidence:
1. <criterion> — <authoritative proof>

Non-goals and authority:
- <excluded outcomes/mechanisms>
- Reads/writes/external/Git authority: <exact scope>

Bead graph:
- Root epic/mol: <existing id or bounded bootstrap rule>
- Initial experiments, if known: <bead → criterion/uncertainty>
- Record notes, scratch, evidence, verdict refs, and dependency/provenance links.

Experiment policy:
- One bead is one RPI experiment.
- Select only work tied to an unmet criterion or named blocking uncertainty.
- Consume each verdict unchanged and apply the ratchet definition.
- Classify discoveries as necessary-now, linked-follow-up, or HOLD/rescope.

Wave envelope:
- RPIs: <positive integer>
- concurrency: <positive integer>
- wall minutes: <positive integer>
- tokens: <positive integer>
- live attempts: <positive integer>

Hard goal envelope:
- total RPIs: <positive integer>
- total wall minutes: <positive integer>
- total tokens: <positive integer>
- total live attempts: <positive integer>
- compactions: <positive integer>
- changed paths: <positive integer>
- No artifact, repair, helper, subject, or wave resets a total.

Breaker and andon:
- Ordinary informative red may produce a materially different next experiment.
- no-ratchet threshold RPIs: <positive integer>
- At that threshold, oscillation, scope pressure, or exhaustion:
  HOLD and consult exactly one bounded fresh helper.
- UNSTUCK resumes with a different experiment; ESCALATE reports NEEDS_OPERATOR.

Wave checkpoint:
- acceptance matrix; graph frontier; verdict/evidence summary;
- ratchets versus non-progress; remaining budgets; next thesis.

Terminal reports:
- ACHIEVED: every terminal criterion is proven.
- NOT_ACHIEVED: envelope or permitted search is exhausted; report exact gaps.
- NEEDS_OPERATOR: judgment, rescope, or helper escalation; stop implementation.
```
