# Executable spec for the /implement skill — the loop's slice executor (BC3 Loop).
# /implement takes ONE tracked issue (a bounded slice carrying domain intent) and
# produces verified git-changes via TDD: a first failing test, then the minimal
# change to green, then refactor — never claiming done without a passing build.
# Hexagon: driving-adapter; consumes: domain; produces: git-changes. (soc-qk4b.2)

Feature: Implement one tracked issue via TDD
  As the loop's slice executor
  I want each issue implemented test-first with a verified build before close
  So that every slice ships as evidence-backed git-changes, never status text

  Background:
    Given a single tracked issue with a bounded scope and stated acceptance
    And a clean worktree at the slice's base SHA

  Scenario: First failing test precedes implementation
    When /implement starts the slice
    Then it writes or identifies a failing test for the acceptance first
    And no implementation code is written before that test fails

  Scenario: Minimal change to green, then refactor
    Given a first failing test
    When /implement makes the change
    Then the test passes with the smallest sufficient change
    And any refactor happens only after green, with the test still passing

  Scenario: Verification iron law gates close
    When /implement believes the slice is done
    Then build, tests, and lint pass (go build/vet/test or the per-language equivalent)
    And the issue closes only after verification passes — never from status text alone

  Scenario: Output is verified git-changes with closure evidence
    Then /implement produces committed git-changes (produces: git-changes)
    And it records ratchet / closure evidence against the issue
