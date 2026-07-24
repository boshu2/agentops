Feature: RPI runs one exact bounded experiment
  @covered-by:skills/rpi/tests/test_run_once.py::test_each_phase_runs_once_and_pass_reports
  Scenario: Core phases run once and stop
    Given one caller intent
    When RPI is invoked
    Then Plan, Implement, and fresh Validate are each dispatched at most once
    And the final artifact is rpi-report.v2
    And it contains no next action

  @covered-by:skills/rpi/tests/test_run_once.py::test_whitespace_and_unicode_bytes_remain_distinct
  Scenario: Exact intent bytes cross every phase
    Given intent bytes with Unicode or whitespace
    When Plan mints the intent
    Then Implement and Validate receive the exact snapshot reference and digest
    And a byte-different representation has a different identity

  @covered-by:skills/rpi/tests/test_run_once.py::test_fail_and_not_proven_report_and_stop
  Scenario: Non-PASS validation is terminal
    Given Validate returns FAIL or NOT_PROVEN
    When RPI reports the verdict
    Then it stops without repair, replan, helper, retry, campaign, or delivery

  @covered-by:skills/rpi/tests/test_run_once.py::test_opaque_correlation_is_preserved_without_interpretation
  Scenario: Opaque correlation crosses without authority
    Given a size-bounded scalar correlation object
    When RPI reports any terminal status
    Then it preserves the correlation unchanged
    And never interprets it as campaign or continuation state
