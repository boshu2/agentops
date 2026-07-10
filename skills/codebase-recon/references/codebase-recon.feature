Feature: Evidence-bounded repository reconstruction

  Scenario: A baseline explains representative repository flows
    Given repository precedence and the current commit are known
    When entry, domain, integration, and test paths are traced
    Then material claims are typed and cited
    And inspected and uninspected scope are explicit

  Scenario: A later run preserves a verified baseline
    Given an earlier recon pack exists
    When the repository is reconstructed again
    Then the earlier baseline is checked against the current commit
    And the new artifact records a delta instead of replacing valid evidence
