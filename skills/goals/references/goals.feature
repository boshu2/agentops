# Executable spec for the /goals skill — the fitness + directive control surface (BC4/BC5).
# /goals maintains and measures the GOALS.md fitness specification: `measure` runs the declared
# gates into a PASS/FAIL verdict; `steer` manages the strategic directives /evolve measures
# against. GOALS.md is the source of truth (output_contract). It consumes no skill — it is the
# measurement root that /evolve consumes. Hexagon: domain; produces result.json; shared-kernel
# with standards. (soc-qk4b)

Feature: Goals maintains and measures the fitness specification
  As the fitness + directive control surface
  I want declared gates measured and directives steered against GOALS.md
  So that the loop has an objective, operator-owned target to compound toward

  @covered-by:tests/e2e/goals-measure-scenarios.sh
  Scenario: measure runs the declared fitness gates into a verdict
    When /goals measure runs
    Then each declared gate is evaluated to PASS or FAIL
    And the result is written to result.json

  @covered-by:tests/e2e/goals-steer-auto.sh
  Scenario: directives are the steering layer the loop measures against
    When directives are managed via /goals steer (add/remove/prioritize)
    Then they live in GOALS.md and surface through `ao goals measure --directives`
    And /evolve selects work against the failing goals + directive gaps

  @covered-by:tests/e2e/goals-trace-chain.sh
  Scenario: GOALS.md is the source of truth
    Then /goals reads and writes GOALS.md (output_contract: GOALS.md)
    And it consumes no other skill — it is the measurement root that /evolve consumes
