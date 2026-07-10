Feature: Evidence-grounded opportunity exploration

  Scenario: A supported portfolio reaches novelty saturation
    Given an open-ended question and readable repository truth
    When opportunity mechanisms are explored and reconciled with existing work
    Then each surviving candidate carries evidence, overlap results, and a behavior scenario
    And the portfolio stops after a pass adds no materially new candidate

  Scenario: Existing coverage leaves no new work
    Given all proposed mechanisms overlap existing behavior or lack support
    When the opportunity portfolio is completed
    Then the result records no new work with overlap evidence
    And no candidate is invented to fill a quota
