# Executable spec for the /push skill — atomic test-commit-push (driving-adapter).
# /push runs the applicable test suites BEFORE committing and pushing, so a failure is caught
# locally instead of reaching the remote. Hexagon: driving-adapter; consumes git-changes;
# produces git-changes. (soc-qk4b)

Feature: Push runs an atomic test-commit-push
  As the validate-commit-push step
  I want the applicable tests run before any commit reaches the remote
  So that a broken change is caught locally, not on origin

  Scenario: the applicable test suites run before committing
    When /push runs
    Then it detects the project type (Go, Python, ...) and runs the matching test suites first

  Scenario: a test failure blocks the push
    When a test suite fails
    Then /push does not commit or push
    And the failure is surfaced locally before it can reach the remote

  Scenario: a clean run commits and pushes
    When all applicable tests pass
    Then /push commits the change and pushes it to the remote
