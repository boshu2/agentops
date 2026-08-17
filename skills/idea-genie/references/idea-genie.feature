Feature: Evidence-grounded opportunity exploration

  @covered-by:tests/scripts/agentops-native-skills.bats::portfolio
  Scenario: A supported portfolio reaches novelty saturation
    Given an open-ended question and readable repository truth
    When opportunity mechanisms are explored and reconciled with existing work
    Then each surviving candidate carries evidence, overlap results, and a behavior scenario
    And the portfolio stops after a pass adds no materially new candidate

  @covered-by:tests/scripts/agentops-native-skills.bats::no-new-work
  Scenario: Existing coverage leaves no new work
    Given all proposed mechanisms overlap existing behavior or lack support
    When the opportunity portfolio is completed
    Then the result records no new work with overlap evidence
    And no candidate is invented to fill a quota

  Scenario: Elicitation stops at declared packet and novelty bounds
    Given an authorized packet of at most 20 sources and 256 KiB
    When Idea Genie reaches three passes, ten candidates, or the output ceiling
    Then it returns the bounded supported portfolio accumulated so far
    And records the limit that stopped exploration without mutating the subject
