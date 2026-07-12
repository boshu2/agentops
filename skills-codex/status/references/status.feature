# Executable behavior for the $status Codex driving adapter.
# The adapter consumes live br, git, gate, and artifact state and produces stdout.

Feature: Status renders resumable AgentOps truth
  As an agent or operator resuming repository work
  I want one evidence-backed dashboard
  So that the next action follows live state rather than conversational memory

  Scenario: live sources produce the three-block dashboard
    Given br, git, reconciliation, and verdict sources are available
    When $status runs after the bounded ao codex ensure-start lifecycle record
    Then it renders Current Work, Latest Gates, and Next Action in that order
    And every displayed value cites a live command or file source

  Scenario: JSON output preserves the same normalized facts
    When $status --json runs
    Then stdout is one dashboard-contract JSON object without explanatory prose
    And coverage names every attempted source as available, unavailable, or malformed

  Scenario: unavailable is distinct from an empty result
    Given an optional or required source cannot be read
    When $status renders the remaining facts
    Then the missing source is marked unavailable in coverage
    And it is not reported as healthy, empty, or none

  Scenario: a negative verdict outranks ordinary continuation
    Given a recent WARN, FAIL, or REFUTED verdict and an in-progress bead both exist
    When $status selects the next action
    Then it selects repair and rerun at priority 1 before resume at priority 2

  Scenario: dashboard probes remain observational
    When $status gathers state after ao codex ensure-start
    Then it does not close work, clean files, start a substrate, or otherwise mutate state
