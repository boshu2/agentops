---
name: scope
description: 'Review a proposed PlanPacket write scope for completeness and ambiguity. Triggers: "review write scope", "check scope boundaries", "scope this change".'
practices:
- ddd-bounded-context
- design-by-contract
hexagonal_role: supporting
consumes:
- proposed-write-scope
produces:
- scope-review
context_rel:
- kind: supplier-to
  with: plan
skill_api_version: 1
context:
  window: isolated
  intent:
    mode: none
  sections:
    exclude: [HISTORY, INTEL, TASK]
  intel_scope: none
metadata:
  capabilities: [scope_review]
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  tier: meta
  dependencies: []
output_contract: 'response: proposed write_scope.include/exclude plus gaps and ambiguities'
---

# /scope — Review a proposed write scope

Review the `write_scope.include` and `write_scope.exclude` fields of a proposed
PlanPacket. This skill is advisory: it does not write a lock, install a hook,
block an edit, claim paths, or change the PlanPacket.

## Inputs

- One active behavior and its acceptance scenarios.
- Proposed include and exclude patterns.
- Known generated companions and fixture/projection paths.
- Explicit non-goals.

## Procedure

1. Map each acceptance criterion to the smallest source paths that may change.
2. Add owned generated companions that must move with those sources.
3. Check whether any include/exclude patterns overlap or are too broad to prove.
4. Identify likely paths the proposal omitted.
5. Return a corrected proposal and the reasons for each change, then stop.

The caller decides whether to adopt the proposal. Plan remains the sole author
of a PlanPacket, and Validate independently compares proven changed paths with
the accepted scope.

## Output

```yaml
write_scope:
  include: ["bounded/source/**"]
  exclude: ["bounded/source/generated-by-other-owner/**"]
generated_companions: ["bounded/generated/**"]
gaps: []
ambiguities: []
```

## Checks

- Patterns are normalized repository-relative paths.
- Includes cover the behavior without granting unrelated directories.
- Excludes do not contradict required changes.
- Generated companions are explicit.
- No ownership, scheduling, Git, hook, retry, release, or delivery state is
  introduced.

## Failure behavior

If the scope cannot be made unambiguous from the supplied acceptance, report
the missing facts and stop. The caller may revise the intent in a new action.
