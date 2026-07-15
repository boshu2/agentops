Feature: RPI runs one bounded experiment
  Scenario: Core phases run once and stop
    Given one intent
    When RPI is invoked
    Then Plan, Implement, and fresh Validate are each dispatched at most once
    And the final report contains no next action

  Scenario: Validation failure does not loop
    Given Validate returns FAIL or NOT_PROVEN
    When RPI reports the verdict
    Then RPI stops without repair, replan, helper, retry, or delivery
