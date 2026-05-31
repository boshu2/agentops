# Executable spec for the /plan skill — vertical-slice decomposition (BC3 Loop).
# /plan consumes dense BDD intent (Discovery output, a bead, or research artifact)
# on the `plan_slices` inbound port and produces a slice-validation plan: one slice
# per Given/When/Then row, each with a first-failing-test target, write scope,
# bounded context, and ownership. Slices group into a wave only when the
# wave-validity check passes. Hexagon: domain; consumes: standards;
# produces: .agents/plans/*.md + execution-packet.json. (soc-qk4b.2)

Feature: Plan converts dense intent into executable slices
  As the loop's decomposition step
  I want dense intent turned into self-contained, validated slices
  So that implementation runs from the plan alone, never from raw chat context

  Scenario: Plan consumes Discovery output
    Given Discovery provides density fields and artifact links
    When Plan receives the `plan_slices` port request
    Then each slice has acceptance criteria, write scope, test levels, and ownership
    And no slice depends on raw Discovery chat context

  Scenario: One slice per Given/When/Then row
    Given a BDD intent issue with N Given/When/Then rows
    When Plan decomposes it
    Then it emits N vertical slices, each with a first-failing-test target

  Scenario: Wave-validity gate before parallelization
    Given a set of candidate slices
    When Plan groups them into a wave
    Then the wave passes only if every row holds: distinct write scopes, no shared
      migration/contract/CLI surface, a declared integration order, an owner per slice,
      and a discard path per slice
    And slices default to sequential when any row fails

  Scenario: Plan produces a durable slice-validation artifact
    Then Plan writes a slice plan to .agents/plans/*.md and an execution-packet.json
    And a fresh agent can execute the slices from those artifacts alone

  # Gherkin acceptance is emitted by default — the operator never hand-specifies BDD (ag-9jle.2).
  Scenario: Every emitted bead carries a Gherkin Scenarios block by default
    Given Plan decomposing a goal into beads
    When a bead is created without the operator asking for BDD
    Then the bead body carries an embedded ## Scenarios (Given/When/Then) block
    And it never ships free-text-only acceptance
    And the ## Scenarios block sits above the acceptance_criteria YAML as the behavior layer
