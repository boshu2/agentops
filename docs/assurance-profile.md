# AgentOps Assurance Profile

AgentOps provides an evidence-producing workflow for AI-agent-created work. It
is an engineering aid, not a certification, approval, release authority, data
boundary, or guarantee of correctness.

## Proven floor

- behavior-first intent with explicit scope and evidence requirements;
- one bounded implementation experiment;
- deterministic content identity through `subject-manifest.v1`;
- one independent judgment from a distinct fresh context;
- a durable `PASS | FAIL | NOT_PROVEN` verdict with checked and unchecked scope.

The freshness attestation is a declared trust fact, not cryptographic proof of
model isolation. A consumer may require stronger runtime controls, multiple
judges, or formal review for high-consequence work.

## Consumer responsibility

The consumer owns model/provider approval, data handling, retention, secrets,
Git and CI policy, tracker state, human approvals, release, rollback, and any
formal assurance mapping. AgentOps packets and verdicts may be inputs to those
systems; they do not replace them.

## Data handling

Treat plans, candidates, evidence, and verdicts as potentially sensitive.
Choose their storage, retention, redaction, and export policy before using them
in a controlled environment. The default verdict directory is local to the
workspace and can be overridden by the caller.
