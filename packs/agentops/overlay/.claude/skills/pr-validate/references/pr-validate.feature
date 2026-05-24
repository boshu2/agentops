# Executable spec for the /pr-validate skill — PR submission-readiness (driving-adapter).
# /pr-validate checks that a PR branch is clean, focused, and ready to submit: isolation,
# upstream alignment, scope containment, and quality gates. Hexagon: driving-adapter; consumes
# validation; produces result.json; customer-of validation. (soc-qk4b)

Feature: PR-validate checks a PR branch is submission-ready
  As the pre-submission PR check
  I want a branch verified for isolation, upstream alignment, scope, and quality
  So that only clean, focused PRs reach the upstream

  Scenario: a PR branch is validated across the readiness dimensions
    When /pr-validate runs on a branch
    Then it checks isolation, upstream alignment, scope containment, and quality gates
    And it produces a PR validation report

  Scenario: scope creep is flagged
    When the branch contains changes outside the contribution's scope
    Then /pr-validate flags the scope-containment failure as not-ready

  Scenario: a failing quality gate blocks readiness
    When a quality gate fails on the branch
    Then /pr-validate reports the branch as not submission-ready
