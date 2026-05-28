# Executable spec for the /quickstart skill — next-action recommender (driving-adapter).
# /quickstart detects the current setup (git, ao, bd, .agents, codex) and recommends the single
# most useful next AgentOps action for that state — orientation, not a fixed script. Hexagon:
# driving-adapter; consumes rpi; produces stdout; customer-of rpi. (soc-qk4b)

Feature: Quickstart shows the next AgentOps action
  As a newcomer or freshly-spawned agent
  I want the current setup detected and one clear next action recommended
  So that I know what to do next without reading the whole repo

  Scenario: setup state is detected
    When /quickstart runs
    Then it detects whether git, ao, bd, and .agents are present

  Scenario: the next action fits the detected state
    When setup detection completes
    Then /quickstart recommends a next action appropriate to that state
    And a repo missing a prerequisite is pointed at installing/initializing it, not at claiming work

  Scenario: a ready repo is pointed at real work
    Given a configured repo with ready beads
    Then /quickstart points at the next concrete step (e.g. ready work via bd / an rpi lifecycle)
