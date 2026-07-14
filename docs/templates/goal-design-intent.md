---
schema_version: 1
kind: goal-design.intent
id: gd-intent-<slug>
slug: <slug>
created_at: "<RFC3339>"
status: draft
objective: "<one sentence human or product outcome>"
why_it_matters: "<problem, value, risk, or strategic reason>"
domain_terms:
  - term: "<term>"
    definition: "<definition>"
    source: "<path-or-url>"
bdd:
  feature: "<capability>"
  scenarios:
    - id: S1
      name: "<observable behavior>"
      given:
        - "<precondition>"
      when:
        - "<action or event>"
      then:
        - "<observable outcome>"
boundaries:
  bounded_context: "<bc-* or repo-local context>"
  in_scope:
    - "<scope item>"
  non_goals:
    - "<non-goal>"
  rollback_or_containment: "<rollback or containment path>"
evidence_for_done:
  first_failing_proof: "<test, gate, or command expected to fail first>"
  validation_command: "scripts/check-goal-design-packet.sh .agents/goal-design/<slug>"
  evidence_path: "<path or glob>"
inputs_to_recheck:
  repo_paths:
    - "<path>"
  prior_artifacts:
    - "<path>"
  live_surfaces:
    - "<surface>"
  stale_assumptions:
    - "<assumption>"
hard_rules:
  - "Keep behavior slices small."
  - "Do not rely on stale claims without verification."
  - "Do not bypass the deterministic packet checker."
---
# Goal Design Intent: <slug>

## Objective

<Rendered from frontmatter.>

## Why It Matters

<Rendered from frontmatter.>

## Domain Terms

- **<term>** - <definition and source>

## BDD Behavior

```gherkin
Feature: <capability>

  Scenario: <one observable behavior>
    Given <precondition>
    When <action or event>
    Then <observable outcome>
    And <secondary proof-relevant outcome>
```

## Boundaries

- Bounded context: `<bc-* or repo-local context>`
- In scope: `<scope item>`
- Non-goals: `<non-goal>`
- Rollback / containment: `<rollback or containment path>`

## Evidence For Done

- First failing proof: `<test, gate, or command>`
- Validation command: `scripts/check-goal-design-packet.sh .agents/goal-design/<slug>`
- Evidence path: `<path or glob>`

## Inputs To Recheck

- Repo paths: `<paths>`
- Prior artifacts: `<paths>`
- Live surfaces: `<surfaces>`
- Assumptions that can go stale: `<assumptions>`

## Hard Rules

- Keep behavior slices small.
- Do not rely on stale claims without verification.
- Do not self-certify acceptance.
