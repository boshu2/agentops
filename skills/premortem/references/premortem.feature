# Executable spec for the /premortem skill — pre-implementation plan gate (domain role).
# /premortem stress-tests a plan BEFORE work starts: it returns a PASS/FAIL verdict on
# the plan and on the wave-validity rows, so a bad plan is sent back to /plan rather than
# executed. Quick mode uses one fresh judge; --deep/--mixed widen the council. Hexagon:
# domain; consumes standards; produces premortem-plan-verdict.v1. (soc-qk4b)

Feature: Pre-mortem stress-tests a plan before implementation
  As the pre-flight gate between slice-planning and TDD
  I want a plan reviewed for failure modes before any code is written
  So that a flawed plan is caught and re-sliced instead of executed

  Scenario: a plan is reviewed and gets a verdict before work starts
    When /premortem runs on a plan or spec
    Then it returns a PASS or FAIL verdict on the plan's failure modes
    And the verdict binds the exact plan path and SHA-256
    And author_id differs from judge_id

  Scenario: wave-validity gates parallelism
    When the plan proposes a parallel wave
    Then premortem checks the wave-validity rows (distinct write scopes, owner per slice, discard path)
    And a wave may run parallel only if every row is conflict-free
    And a FAIL sends the plan back to /plan to re-slice (or run sequential)

  Scenario: Between-wave Premortem runs only for a materially changed plan
    Given targeted wave evidence changed acceptance, dependencies, write scope, or risk
    And the orchestrator wrote the changed plan
    When Premortem runs before the next wave
    Then it judges that exact changed plan
    And Validate and Learn did not invoke Premortem directly

  Scenario: unchanged plan inputs reuse the bound verdict
    Given the accepted plan digest, acceptance, dependencies, write scope, and risk are unchanged
    When another tranche wave is considered
    Then the existing Premortem verdict is reused
    And no new judge, council, report, or registry write is created

  Scenario: quick mode uses one fresh judge by default
    When /premortem runs without --deep/--mixed/--debate
    Then it runs exactly one fresh-context judge distinct from the author
    And it does not start a council
    And optional council fan-out cannot substitute for the exact-plan verdict

  Scenario: a failure reports every blocker once
    Given the plan has concrete readiness defects
    When the fresh judge returns FAIL
    Then blockers_complete is true
    And the blocker list is nonempty and evidence-bound
    And Premortem owns no retry, budget, helper, or delivery state
