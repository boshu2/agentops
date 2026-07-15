---
name: scope
description: Review the bead or caller intent write scope
---
# Scope — Review a proposed write scope

Review the write scope in the existing bead or caller intent. This skill is
advisory: it does not create a second planning artifact, write a lock, install
a hook, block an edit, or claim paths.

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

The caller decides whether to adopt the proposal in the original intent source,
and Validate independently compares runtime-derived changed paths with that scope.

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
