SKILL GUIDANCE (loaded): security — run repeatable repository security checks
and report findings PLUS coverage gaps.

Critical constraints (the load-bearing rules):
- Treat missing or errored scanners as a COVERAGE GAP, never as a clean
  finding. Absent evidence is not evidence of absence.
- A status report must state what was scanned AND what was not; a "clean"
  status is only available when the intended scan surface actually ran.
- Keep collection read-only; never mutate policy or baselines to manufacture
  green.
- Report the scan once: findings plus coverage gaps. Remediation and risk
  acceptance are caller decisions.
