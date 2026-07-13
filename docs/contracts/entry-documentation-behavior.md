# Entry Documentation Behavior Contract

> Judgment contract for the repository's public first-value documentation.
> Agents judge meaning in the actual documents; deterministic gates verify only
> facts such as artifact identity, links, command existence, and verdict shape.

## Intent

A newcomer should leave the entry documentation with one accurate first action:
use the skill-driven operating loop. Historical or specialist surfaces may be
named, but they must not be presented as the product's current front door.

## Artifact set

Pin the commit or content digest of all four documents before judging:

- `README.md`
- `docs/index.md`
- `docs/getting-started/index.md`
- `docs/first-value-path.md`

## Acceptance scenarios

```gherkin
Feature: Entry documentation routes a newcomer to current first value

  Scenario: A newcomer identifies the current first action
    Given the newcomer reads the four entry documents as one journey
    When they identify what to do first
    Then they choose the skill-driven operating loop
    And they can distinguish planning, implementation, validation, and proof

  Scenario: Retired surfaces remain historical or specialist
    Given an entry document mentions ao factory start, ao verify, /vibe, or a council packet
    When the newcomer interprets the recommendation in context
    Then none is presented as the current product front door or default first-value path
    And any retained mention has a clear historical, specialist, or supporting role

  Scenario: The journey agrees across entry points
    Given a newcomer enters through README, the site index, getting started, or first value
    When they follow the recommended path
    Then the documents do not send them to contradictory starting workflows

  Scenario: A hypothetical phrase is not an artifact defect
    Given a judge can imagine alternate wording that would violate this contract
    When that wording is absent from the pinned artifact set
    Then it is a proposed holdout or follow-up, not a blocking finding
```

## Judge contract

Run `validate --mode=pre-impl --target=scenario` with two disposable,
context-isolated judges. Each receives only:

1. this contract;
2. the pinned four-document artifact set;
3. the executable/generated command surface needed to resolve factual disputes;
4. the read-only verdict schema and output path.

A blocking finding must satisfy every condition:

- it cites an exact passage in the pinned artifacts;
- it identifies the acceptance scenario that passage violates;
- it describes a material newcomer decision that would be wrong or ambiguous;
- it survives independent reproduction or reconciliation by the second judge.

Generated counterexamples, stylistic preferences, and absent hypothetical
phrases are nonblocking. Record useful ones as holdouts; do not mutate the
acceptance contract during the review.

## Deterministic prechecks

Deterministic tooling may prove only:

- all four files exist and their digest/commit is pinned;
- Markdown links resolve;
- commands quoted as executable exist in the generated CLI surface;
- the verdict artifacts match `schemas/verdict.v1.schema.json`;
- judge identity differs from author identity when independence is claimed.

No regex, keyword window, or parser may claim to decide whether prose is
misleading, historical, primary, contradictory, or semantically correct.

## Verdict

Default validation uses two independent judges:

- **PASS:** both judges find every scenario satisfied.
- **WARN:** the actual journey is correct but a nonblocking ambiguity or coverage gap remains.
- **FAIL:** a material, artifact-present scenario violation survives reconciliation.

Landing remains a separate commit-bound pawl. A documentation-behavior PASS
does not authorize unrelated code or release changes.
