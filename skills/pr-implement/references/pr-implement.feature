# Executable spec for the /pr-implement skill — fork-based OSS contribution (driving-adapter).
# /pr-implement executes a contribution plan on a fork branch with a MANDATORY isolation check
# before and during implementation, keeping the PR clean and scoped. Hexagon: driving-adapter;
# consumes crank; produces git-changes; customer-of crank. (soc-qk4b)

Feature: PR-implement executes a scoped contribution on a fork with isolation
  As the fork-based OSS implementation step
  I want a contribution plan executed on a fork branch under isolation checks
  So that the resulting PR is clean, focused, and free of unrelated changes

  Scenario: a contribution plan is implemented on a fork branch
    Given a plan artifact from /pr-plan (or a repo URL)
    When /pr-implement runs
    Then it produces the code changes on a fork branch

  Scenario: the isolation check is mandatory before and during
    When /pr-implement implements the change
    Then it runs an isolation check before and during implementation
    And changes outside the contribution's scope are kept out of the PR

  Scenario: the output stays focused
    When implementation completes
    Then the fork branch contains only the scoped contribution, not unrelated edits
