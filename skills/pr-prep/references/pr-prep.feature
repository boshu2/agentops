# Executable spec for the /pr-prep skill — PR preparation (driving-adapter).
# /pr-prep prepares a contribution: it learns the target repo's conventions, validates the
# tests, and generates a properly-formatted PR body on a branch. It is convention-matching,
# not boilerplate. Hexagon: driving-adapter; consumes domain; produces git-changes;
# customer-of domain. (soc-qk4b)

Feature: PR-prep produces a convention-matching PR body and branch
  As the PR-preparation step
  I want the target repo's conventions learned and tests validated before a PR body is written
  So that the contribution matches the repo and is backed by passing tests

  Scenario: the target repo's conventions are analyzed
    When /pr-prep runs against a repo
    Then it analyzes the repo's git history and commit/PR conventions
    And the generated PR body and commits follow those conventions

  Scenario: tests are validated before the PR body is generated
    When /pr-prep prepares the contribution
    Then it validates the tests first
    And it does not generate the PR body as if tests passed when they did not

  Scenario: a PR body and branch are produced
    When preparation completes
    Then it produces a properly-formatted PR body (markdown) on a git branch
