# Executable spec for the /discovery skill — front-of-loop intent densifier (BC3 Loop).
# /discovery runs the artifact-first research → plan DAG and hands dense intent across
# the plan_slices port, producing an execution packet — it never inlines the Plan
# decomposition in its own prose. Promoted from the inline Feature block in SKILL.md.
# (soc-qk4b.2)

Feature: Discovery hands dense intent to planning
  As the front of the loop
  I want research and design densified and handed cleanly to Plan
  So that planning receives artifact links + density fields, not re-derived prose

  Scenario: Discovery delegates to Plan
    Given Discovery has a goal, research path, and design or brainstorm evidence
    When it crosses the `plan_slices` port
    Then it sends density fields and artifact links
    And it does not inline the Plan decomposition in Discovery prose

  Scenario: Discovery produces a durable execution packet
    When the discovery DAG completes
    Then it writes a JSON execution packet on disk for the next loop phase
    And the packet carries the goal, research, and design artifact references

  # Gherkin acceptance is emitted by default — the operator never hand-specifies BDD (ag-9jle.2).
  Scenario: Discovery requires every planned bead to carry Gherkin scenarios by default
    Given Discovery crosses the plan_slices port to /plan
    When /plan returns beads at STEP 4
    Then every bead carries an embedded ## Scenarios (Given/When/Then) block
    And Discovery sends any bead with free-text-only acceptance back to /plan before compiling the packet

  # Open-ended path — generate-winnow → operationalize → refine (ag-yw0).
  # Additive to the default flow above; strict delegation is preserved.
  # Documentation-only spec (this file is allowlisted in
  # scripts/.scenario-linkage-allow); wiring is asserted by the bats regression
  # test tests/scripts/brainstorm-discovery-ideation.bats.

  Scenario: an open-ended goal takes the generate-winnow path
    Given an open-ended goal like "improve the project" or the --ideate flag
    When /discovery runs
    Then it delegates to /brainstorm in ideation mode as a separate skill invocation
    And it does not inline the 30-idea generation in Discovery prose
    And the default specific-goal flow remains unchanged

  Scenario: Discovery operationalizes the winnowed portfolio into self-documenting beads
    Given a ranked portfolio of 15 ideas from ideation mode
    When the operationalize step runs
    Then it creates self-documenting br beads with dependency structure and explicit test tasks
    And each bead carries what, why, how, risks, and success criteria
    And it uses br for tracking and bv for graph triage

  Scenario: Discovery refines beads in plan space before crank
    Given operationalized beads exist
    When the refine step runs
    Then it makes 4-5 refinement passes re-reading AGENTS.md each pass
    And it does not oversimplify or lose features or functionality
    And it validates no dependency cycles before handing the packet to /crank
