---
name: learn
description: 'Optionally analyze collections of durable verdicts for recurring evidence after the critical path. Triggers: "learn from verdicts", "mine validation history".'
practices:
- continuous-learning
- evidence-based-engineering
hexagonal_role: supporting
consumes:
- verdict.v2
produces:
- learning-observations
context_rel:
- kind: customer-of
  with: validate
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: false
  tier: execution
  dependencies: []
  capabilities: [analyze_verdict_collections]
  effects: [read_durable_verdicts, delete_expired_advisory_observations, optionally_write_advisory_observations]
  canonical_status: canonical
  disposition: keep_off_path
output_contract: inline observations or learning-observations.v1 JSON with source digests, created_at, expires_at, and cleanup effects
---

# Learn

## Constraints

- **Why advisory memory must expire.** Durable output is `learning-observations.v1` JSON under the exact physical
  root `.agents/scratch/learn/`. Every artifact records UTC `created_at` and
  `expires_at`; default TTL is 7 days and the caller may choose 1–30 days.
  Invalid, absent, or longer expiry is rejected before write.
- **Why expiry needs real cleanup.** At the start of an authorized Learn invocation, run
  `scripts/prune-expired.sh --apply` with the caller authorization ID. It scans
  at most 1,000 direct entries for 10 seconds, considers only regular `*.json`
  children, never follows symlinks, and removes only recognized artifacts whose fixed-format expiry is
  at or before `now`. Directories, links, unknown JSON, and live files stay.
- A cleanup parse, limit, deadline, race, or deletion failure is reported and
  stops before a new durable write. The output lists expired paths deleted,
  live/unknown paths retained, and failures; inline mode still performs and
  reports the cleanup when the scratch root exists.

Learn is an optional, off-path consumer of durable `verdict.v2` collections.
It may summarize recurring evidence and propose a candidate deterministic check
for later human or caller evaluation.

Learn does not run during RPI, validate a subject, alter a verdict, mutate a
plan, promote a rule, choose continuation, or mint lifecycle artifacts. Missing
Learn output never changes whether a candidate is valid.

When invoked, bind every observation to verdict and finding digests, distinguish
repeated objectives from repeated reviews of one objective, disclose the sample
size, and stop at advisory evidence.

Overweight failures: a `NOT_PROVEN` or `FAIL` verdict carries more teaching
value than a PASS, because it names a rule the loop lacked. Harvest kernels
from failed lanes first — the canonical example is the mutating-check
quarantine in `skills/validate/SKILL.md`, a durable rule minted from a
`NOT_PROVEN`-then-`PASS` verdict pair.

Prune for provenance decay: every cited artifact must still resolve — the
file exists or the verdict digest is present under `.agents/ao/verdicts/`. A
citation that no longer resolves gets pruned rather than paraphrased, and
confidence in a lesson that has not been reproduced since its source decayed
goes down, not sideways.

When the caller asks for a durable artifact, write the observations under
`.agents/scratch/learn/<run-id>.json`, validate it with
`scripts/validate-output.sh`, and return the path; otherwise return them inline.
The write is advisory and expires — it is never a source of record, and its
absence never changes whether a candidate is valid. Completion means cleanup
finished, every observation cites still-resolving digests, and durable output
has a validated expiry no more than 30 days away.
