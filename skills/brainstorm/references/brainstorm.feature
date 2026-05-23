# Executable spec for the /brainstorm skill — goal clarification (domain role).
# /brainstorm separates WHAT from HOW: it explores the problem space and captures testable
# Gherkin acceptance examples BEFORE planning commits to a solution. It runs upstream of loop
# move 1 (shape intent as BDD). Hexagon: domain; consumes standards; produces result.json +
# verdict.json; shared-kernel with standards. (soc-qk4b)

Feature: Brainstorm separates goals from implementation before planning
  As the pre-planning goal-clarification step
  I want a free-text goal explored as a problem space, then captured as Gherkin
  So that planning starts from clear, testable intent rather than a premature solution

  Scenario: a goal is clarified through the four phases
    When /brainstorm runs on a free-text goal
    Then it works through assess-clarity → understand-idea → explore-approaches → capture-design

  Scenario: approaches are explored as options, not a single solution
    When the explore phase runs
    Then it generates multiple options, compares tradeoffs, and applies adversarial critique
    And it separates the problem (WHAT) from any one solution (HOW)

  Scenario: capture writes testable Gherkin examples
    When the capture phase completes
    Then it produces Given/When/Then acceptance examples for /plan and /discovery
    And capture is not complete until at least one happy path and one critical edge are written
