# Executable spec for the /beads skill — the mandatory issue tracker (BC, driven-adapter).
# /beads is the bd-backed, dependency-aware tracker every process skill records work in — NOT
# TaskCreate or markdown TODOs. Issues are created before code, ready-work is detected by
# dependency, discovered work links back, and live bd reads are authoritative. Hexagon:
# driven-adapter; consumes/produces bd-issue; supplier-to crank. (soc-qk4b)

Feature: Beads is the mandatory dependency-aware issue tracker
  As the work-tracking adapter
  I want all issue tracking to go through bd, dependency-aware and Dolt-backed
  So that work is created-before-code, unblocked work is detectable, and discovery is linked

  Scenario: bd is the tracker, not TaskCreate or markdown
    When work needs tracking
    Then it is recorded as a bd issue (created before the code), not a TaskList or markdown TODO

  Scenario: ready work is detected by dependency
    When an agent asks what to work on
    Then `bd ready` surfaces only unblocked issues
    And the agent claims one atomically before starting

  Scenario: discovered work links back to its origin
    When new work is found mid-task
    Then it is filed with a discovered-from dependency to the parent issue

  Scenario: live bd reads are authoritative
    Then `bd show`/`bd ready`/`bd list` are the decision source
    And `.beads/issues.jsonl` is not treated as primary when live bd data is available
