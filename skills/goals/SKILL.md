---
name: goals
description: 'Compatibility alias — renamed to fitness. Use fitness to measure declared project fitness goals. Triggers: "goals" (deprecated).'
practices:
- dora-metrics
hexagonal_role: domain
consumes: []
produces:
- goal-measurement-report
- optional-goal-baseline-snapshot
- optional-rendered-goal-spec
context_rel:
- kind: alias-of
  with: fitness
skill_api_version: 1
user-invocable: true
metadata:
  capabilities: [fitness_compatibility_alias]
  effects: [read_goals_source, read_goal_history_and_evidence, optionally_write_goal_snapshot, optionally_write_rendered_spec]
  canonical_status: canonical
  disposition: keep_off_path
  tier: product
  dependencies: []
output_contract: fitness output unchanged — report plus observed reads/writes and any declared snapshot/render artifact
---
# Goals — compatibility alias for fitness

This skill was renamed to `fitness` on 2026-07-29. Everything it did lives
there unchanged; the `ao goals` CLI command family keeps its name.

When invoked, apply `skills/fitness/SKILL.md` exactly as written — this
alias adds no behavior, grants no authority, and produces nothing of its
own. It exists so existing references and habits keep resolving, and it is
deliberately not advertised as a destination.

The alias declaration repeats Fitness's transitive reads and optional writes so
routers and operators do not mistake an alias invocation for read-only work:
`measure`/`drift`/`export` may write the fixed baseline snapshot, while
`render --out` may write one caller-approved derived spec. The returned output
is Fitness's honest report with `reads`, `writes`, and `stdout`; there is no
second alias artifact.

Physical deletion follows the observed-zero policy: this alias is removed
only after a declared observation window shows no remaining use.
