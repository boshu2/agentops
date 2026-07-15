Feature: Implement runs one bounded experiment
  Scenario: Behavior change follows RED GREEN refactor
    Given one PlanPacket
    When Implement changes the subject
    Then the first acceptance check fails for the expected missing behavior
    And the smallest change makes it green
    And refactoring preserves the acceptance test

  Scenario: Incomplete changed path coverage stays honest
    Given complete changed paths cannot be established
    Then CandidatePacket records changed_path_coverage_complete as false
    And Implement does not infer missing paths
