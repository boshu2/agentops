# Executable spec for the /ship-loop skill — coherent-arc fast lane (driving-adapter).
# /ship-loop drives ONE coherent arc (a closable bead or small-epic slice) from claim through
# squash-merge: claim → test → impl → pre-push → push → squash auto-merge → close. It is the
# bot-paired fast lane, not for large epics. Hexagon: driving-adapter; consumes beads + rpi +
# post-mortem; produces git-changes + merged-prs; customer-of rpi. (soc-qk4b)

Feature: Ship-loop drives a coherent-arc PR from claim to merge
  As the fast-lane PR cycle
  I want one coherent arc taken claim-to-merge with the gates enforced
  So that small internal changes ship without manual step-by-step driving

  Scenario: the full cycle runs end to end
    When /ship-loop runs on a claimed bead
    Then it proceeds claim → test → impl → pre-push → push → squash auto-merge → close

  Scenario: scope is one coherent arc, not a large epic
    Given a unit of work
    Then /ship-loop accepts one closable bead or small-epic slice (a single rollback unit)
    And a large epic is routed elsewhere (sliced into multiple PRs), not run as one ship-loop

  Scenario: merge is gated and the bead is closed
    When CI is green
    Then the PR is squash auto-merged and the bead is closed
    And the run produces git-changes and a merged PR
