# Executable spec for the /premortem skill — pre-implementation plan gate (domain role).
# /premortem stress-tests a plan BEFORE work starts: it returns a PASS/WARN/FAIL verdict on
# the plan and on the wave-validity rows, so a bad plan is sent back to /plan rather than
# executed. It runs inline (--quick) by default; --deep/--mixed widen the council. Hexagon:
# domain; consumes standards; produces result.json + verdict.json. (soc-qk4b)

Feature: Pre-mortem stress-tests a plan before implementation
  As the pre-flight gate between slice-planning and TDD
  I want a plan reviewed for failure modes before any code is written
  So that a flawed plan is caught and re-sliced instead of executed

  Scenario: a plan is reviewed and gets a verdict before work starts
    When /premortem runs on a plan or spec
    Then it returns a PASS/WARN/FAIL verdict on the plan's failure modes
    And the verdict is written to verdict.json (council schema)

  Scenario: wave-validity gates parallelism
    When the plan proposes a parallel wave
    Then premortem checks the wave-validity rows (distinct write scopes, owner per slice, discard path)
    And a wave may run parallel only if every row is conflict-free
    And a FAIL sends the plan back to /plan to re-slice (or run sequential)

  Scenario: Between-wave Premortem receives an orchestrator-owned changed plan
    Given Validate has handed its verdict to Learn
    And the orchestrator accepted a material plan impact and changed the plan
    When Premortem runs before the next wave
    Then it judges that exact changed plan
    And Validate and Learn did not invoke Premortem directly

  Scenario: quick mode is the inline default
    When /premortem runs without --deep/--mixed/--debate
    Then it runs inline (--quick) as a single-agent structured review, no council spawning
    And --deep/--mixed/--debate widen the council fan-out
