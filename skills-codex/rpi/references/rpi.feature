Feature: RPI runs one bounded experiment
  @covered-by:skills/rpi/tests/test_run_once.py::test_each_phase_runs_once_and_pass_reports
  Scenario: Core phases run once and stop
    Given one intent
    When RPI is invoked
    Then Plan, Implement, and fresh Validate are each dispatched at most once
    And the final report contains no next action

  @covered-by:skills/rpi/tests/test_run_once.py::test_fail_reports_and_stops_without_another_dispatch
  Scenario: Validation failure does not loop
    Given Validate returns FAIL or NOT_PROVEN
    When RPI reports the verdict
    Then RPI stops without repair, replan, helper, retry, or delivery

  @covered-by:skills/rpi/scripts/validate.sh
  Scenario: Interactive output summarizes the machine artifact
    Given RPI has produced an rpi-report.v1 machine artifact
    When RPI responds to an interactive caller
    Then the response leads with status and the caller-visible outcome
    And it includes only the strongest proof, material unchecked scope, and verdict link
    And the full schema object is emitted only when machine-readable output was requested
