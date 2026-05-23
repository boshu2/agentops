# Executable spec for the /session-bootstrap skill — universal init prompt (driving-adapter).
# Every agent spawned into an AgentOps repo runs `ao session bootstrap` FIRST, getting the same
# orientation report regardless of model — the contract that makes model fungibility operational.
# Hexagon: driving-adapter; consumes bd + onboard; produces stdout + json; customer-of AGENTS.md.
# (soc-qk4b)

Feature: Session-bootstrap gives every agent the same orientation frame
  As the universal init step for any agent in the repo
  I want every agent to start from one identical orientation report
  So that a Claude agent and a Codex agent can be swapped on any bead without re-orienting

  Scenario: every agent runs bootstrap first and gets an identical frame
    When an agent is spawned into an AgentOps repository
    Then it runs `ao session bootstrap` before claiming work
    And the orientation report is identical regardless of which model is running

  Scenario: the SessionStart hook fires fail-open
    When the SessionStart hook runs bootstrap automatically
    Then it runs `ao session bootstrap --robot` and discards the exit code
    And a bootstrap failure never blocks the session from starting

  Scenario: pipeline and headless agents get machine-readable output
    When a headless or pipeline agent bootstraps
    Then `ao session bootstrap --json` provides the orientation as structured JSON before it claims work
