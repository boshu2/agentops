# Executable spec for the /scenario skill — holdout scenario management (BC4 Evidence & Trust).
# /scenario creates, validates, and lists holdout scenarios under .agents/holdout/, links
# them to GOALS.md directives, and feeds them into /validation's behavioral checks. Hexagon:
# supporting; consumes: scenario definitions + the scenario schema; produces: validated
# holdout scenario artifacts in .agents/holdout/*.json. (soc-qk4b)

Feature: Scenario manages holdout scenarios for behavioral validation
  As an agent guarding against regressions
  I want holdout scenarios authored, schema-validated, and linked to goals
  So that /validation can score behavior against a held-out set

  Background:
    Given a holdout directory under .agents/holdout/

  Scenario: Scenarios are authored into the holdout directory
    When /scenario authors a scenario
    Then it writes a scenario artifact under .agents/holdout/

  Scenario: Scenarios are validated against the schema
    When /scenario validates
    Then each scenario is checked against the scenario schema and failures are reported

  Scenario: Scenarios are listable and linkable to GOALS directives
    When /scenario lists
    Then it shows the holdout scenarios and their links to GOALS.md directives

  Scenario: Holdout scenarios feed validation
    When /validation runs
    Then it consumes the holdout scenarios for behavioral scoring
