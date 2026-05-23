# Executable spec for the /hooks-authoring skill — runtime hook authoring (BC5 Runtime).
# /hooks-authoring creates or reviews AgentOps runtime hooks with portable behavior: a hook
# script, its manifest entry, tests, and validation evidence — so a hook is a bounded,
# tested adapter rather than ad-hoc glue. Hexagon: domain; consumes: a hook intent + the
# hooks manifest contract; produces: hook script + manifest entry + tests + evidence. (soc-qk4b)

Feature: Hooks-authoring produces bounded, tested runtime hooks
  As an author extending the runtime
  I want a hook delivered with its manifest entry, tests, and evidence
  So that hooks are bounded adapters, not unaudited glue

  Background:
    Given a hook intent and the hooks manifest contract

  Scenario: A hook script is authored for the right event
    When /hooks-authoring creates a hook
    Then it produces a hook script bound to the correct lifecycle event

  Scenario: The hook is registered in the manifest
    When the hook script exists
    Then a matching manifest entry is added so the runtime loads it

  Scenario: The hook ships with tests and validation evidence
    When authoring completes
    Then it produces tests and validation evidence for the hook's behavior

  Scenario: Review mode checks an existing hook for portability
    When /hooks-authoring reviews an existing hook
    Then it checks portability and bounded behavior rather than re-authoring it
