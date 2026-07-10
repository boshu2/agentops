---
name: pattern-mining
description: 'Test repeated implementation shapes against independent exemplars and a holdout before routing an earned abstraction. Triggers: "mine a recurring code pattern", "is this abstraction earned", "extract invariants from implementations".'
practices:
- design-patterns
- legacy-code-seams
- design-by-contract
hexagonal_role: supporting
consumes:
- repo-context
- task-question
produces:
- pattern-mining.v1
context_rel:
- kind: customer-of
  with: research
- kind: customer-of
  with: validate
- kind: supplier-to
  with: operationalize
skill_api_version: 1
user-invocable: true
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: execution
  dependencies:
  - operationalize
  - validate
output_contract: pattern-mining.v1 JSON validated by skills/pattern-mining/scripts/validate-output.sh
---

# Pattern Mining

Decide whether repeated code demonstrates a reusable rule or only a plausible
hypothesis. Similar names and syntax are not enough; the abstraction must
survive examples it was not designed around.

## Workflow

1. State the candidate pattern and collect independently implemented
   exemplars with repository anchors. Use `research` when coverage is unclear.
2. From the exemplars, separate required invariants, legitimate variation
   points, and incidental similarity.
3. Require at least three distinct exemplars before promotion is possible.
   Form the candidate abstraction without using the holdout.
4. Test it against every exemplar, then a separate holdout. Back-apply the
   refined abstraction to the original exemplars so the holdout fix cannot
   silently break them.
5. Emit `outcome: promote` only when the exemplar floor, holdout, and
   back-application all pass. Route that evidence to `operationalize`, which
   decides whether the eventual shape is a skill, gate, library, template, or
   no action.
6. Otherwise emit `outcome: hypothesis` with `route: no-action`. Keep the
   evidence bounded and name what additional observation would retest it.

```bash
skills/pattern-mining/scripts/validate-output.sh <pattern.json>
```

This skill owns evidence for the pattern. It never creates the reusable
artifact itself and never promotes a failed or untested holdout.

Executable behavior:
[references/pattern-mining.feature](references/pattern-mining.feature).

## Do not

- Count copies from one implementation lineage as independent exemplars.
- Hide variation by calling it incidental.
- Route a hypothesis directly to a skill, rule, gate, or library.
