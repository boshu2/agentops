# AgentOps Incident Runbook

## Preserve evidence

Copy the resolved intent or snapshot, subject manifest, check receipts, command
output, and verdict before changing the subject. Do not treat incomplete or
corrupt evidence as PASS.

## Classify the boundary

- `FAIL`: a concrete acceptance defect was proven.
- `NOT_PROVEN`: identity, freshness, scope coverage, evidence, or persistence
  was insufficient.
- deterministic command failure: the repository check failed independently of
  semantic validation.
- installation drift: generated projections or installed skill links differ
  from their source metadata.

AgentOps reports and stops. The operator or calling system chooses recovery,
revision, rollback, Git, CI, and release actions.

## Repository diagnostics

```bash
python3 scripts/check-cathedral-cut-conformance.py
scripts/regen-all.sh --check
cd cli && go test ./...
```

Run only the command needed to reproduce the observed failure before expanding
to the full deterministic suite.

## Escalation

Escalate to the operator when authority, credentials, safety judgment, or an
external system change is required. A failed check or reviewer disagreement is
evidence to report, not an AgentOps-controlled retry state.
