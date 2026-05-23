# Executable spec for the /status skill — work-status dashboard (driving-adapter).
# /status reports current AgentOps work state — its primary source is the bd tracker
# (ready/in-progress/epics) augmented with ratchet, flywheel, and git state — as a human
# dashboard or machine-readable JSON, degrading gracefully when a tool is missing. Hexagon:
# driving-adapter; consumes bd; produces stdout. (soc-qk4b)

Feature: Status shows the AgentOps work dashboard
  As an agent or operator orienting in a repo
  I want current work state surfaced from the tracker in one view
  So that I can see ready/in-progress work and project health at a glance

  Scenario: the dashboard reports work state from bd
    When /status runs
    Then it reports work state from bd (ready, in-progress, open epics)
    And it augments that with ratchet, flywheel, and git state

  Scenario: --json gives machine-readable output
    When /status --json runs
    Then it emits the same status as structured JSON

  Scenario: missing tools degrade gracefully
    When a data source (ratchet/flywheel) is unavailable
    Then /status marks that section unavailable and still renders the rest, without crashing
