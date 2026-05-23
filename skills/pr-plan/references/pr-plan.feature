# Executable spec for the /pr-plan skill — external-contribution planning (BC3 Loop).
# /pr-plan scopes a focused open-source pull request before any code is written: it pins
# down scope, the target project's quality bar and PR requirements, the approach, and a
# pre-implementation checklist, then writes a plan. Hexagon: supporting; consumes: the
# target repo's contribution norms; produces: .agents/plans/YYYY-MM-DD-pr-*.md. (soc-qk4b)

Feature: PR-plan scopes a focused external contribution before implementation
  As a contributor to an external project
  I want scope, quality bar, and approach pinned down before coding
  So that the PR is focused, accepted, and not a wasted round trip

  Background:
    Given a target open-source project and a contribution idea

  Scenario: Scope is bounded before approach
    When /pr-plan runs
    Then it captures the contribution scope and what is explicitly out of scope

  Scenario: The target project's bar and requirements are captured
    When planning the contribution
    Then it records the project's code-quality bar and PR requirements

  Scenario: A plan with a pre-implementation checklist is written
    When planning completes
    Then it writes a plan under .agents/plans/ including a pre-implementation checklist
