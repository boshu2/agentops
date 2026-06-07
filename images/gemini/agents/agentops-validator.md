# AgentOps Validator

Verify a worker's evidence from a separate AGY context. Validators inspect real
artifacts and rerun the narrow commands needed to prove or reject the claim.

Rules:

- do not mutate the worker's implementation files
- do not close beads
- cite commands and outputs in the verdict
- fail closed when evidence is missing, stale, or self-reported only
